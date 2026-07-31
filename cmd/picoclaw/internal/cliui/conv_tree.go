package cliui

import (
	"fmt"
	"strconv"
	"strings"
)

// ConvNode is one node in the nested conversation tree.
//
// Layout rules:
//   - Root level nodes are sessions.
//   - Session children are requests (every session starts with a request).
//   - Request children are responses (0..1 typical; keep slice for flexibility).
//   - Response children are follow-up requests (0..n — branching).
type ConvNode struct {
	ID       string
	Kind     TreeRowKind
	Session  string // session key (all nodes)
	Content  string // request/response body; session title source
	Children []*ConvNode
	Parent   *ConvNode `json:"-"`
}

// nextID is used when allocating node IDs within a forest.
type idGen struct{ n int }

func (g *idGen) next(session string) string {
	g.n++
	return session + "#" + strconv.Itoa(g.n)
}

// BuildConvTreeFromHistory nests a linear chat history under a session root:
//
//	Session
//	  ↑ req1
//	    ↓ resp1
//	      ↑ req2
//	        ↓ resp2
func BuildConvTreeFromHistory(sessionKey, title string, msgs []ChatMessage, gen *idGen) *ConvNode {
	if gen == nil {
		gen = &idGen{}
	}
	root := &ConvNode{
		ID:      gen.next(sessionKey),
		Kind:    TreeRowSession,
		Session: sessionKey,
		Content: title,
	}
	var tip *ConvNode = root // attach next request here (session or response)
	for _, turn := range PairTurns(msgs) {
		if turn.Request == "" && turn.Response == "" {
			continue
		}
		var req *ConvNode
		if turn.Request != "" {
			req = &ConvNode{
				ID:      gen.next(sessionKey),
				Kind:    TreeRowRequest,
				Session: sessionKey,
				Content: turn.Request,
				Parent:  tip,
			}
			tip.Children = append(tip.Children, req)
		}
		if turn.Response != "" {
			parent := req
			if parent == nil {
				// Orphan assistant: hang under tip as synthetic empty request's response.
				parent = tip
			}
			resp := &ConvNode{
				ID:      gen.next(sessionKey),
				Kind:    TreeRowResponse,
				Session: sessionKey,
				Content: turn.Response,
				Parent:  parent,
			}
			parent.Children = append(parent.Children, resp)
			tip = resp // next request nests under this response
		} else if req != nil {
			// Pending request with no response yet — next request would be sibling under tip's parent;
			// keep tip as session/response parent for another sibling request.
			tip = req.Parent
			if tip == nil {
				tip = root
			}
		}
	}
	if root.Content == "" || root.Content == "(empty)" {
		root.Content = sessionTitleFromTree(root)
	}
	return root
}

func sessionTitleFromTree(root *ConvNode) string {
	if root == nil {
		return "(empty)"
	}
	var last string
	var walkLast func(*ConvNode)
	walkLast = func(n *ConvNode) {
		if n.Kind == TreeRowRequest && strings.TrimSpace(n.Content) != "" {
			last = n.Content
		}
		for _, c := range n.Children {
			walkLast(c)
		}
	}
	walkLast(root)
	if last == "" {
		return "(empty)"
	}
	return last
}

// PathMessages returns user/assistant messages from session root down to node (inclusive).
func PathMessages(node *ConvNode) []ChatMessage {
	if node == nil {
		return nil
	}
	var chain []*ConvNode
	for n := node; n != nil; n = n.Parent {
		chain = append(chain, n)
	}
	// reverse to root→leaf
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	out := make([]ChatMessage, 0, len(chain))
	for _, n := range chain {
		switch n.Kind {
		case TreeRowRequest:
			out = append(out, ChatMessage{Role: "user", Content: n.Content})
		case TreeRowResponse:
			out = append(out, ChatMessage{Role: "assistant", Content: n.Content})
		}
	}
	return out
}

// PathMessagesBefore returns messages along the path to parent (exclusive of a new child).
func PathMessagesBefore(parent *ConvNode) []ChatMessage {
	return PathMessages(parent)
}

// AttachRequest adds a request under parent (session or response).
func AttachRequest(parent *ConvNode, content string, gen *idGen) *ConvNode {
	if parent == nil {
		return nil
	}
	if gen == nil {
		gen = &idGen{}
	}
	req := &ConvNode{
		ID:      gen.next(parent.Session),
		Kind:    TreeRowRequest,
		Session: parent.Session,
		Content: content,
		Parent:  parent,
	}
	parent.Children = append(parent.Children, req)
	return req
}

// AttachResponse adds a response under a request node.
func AttachResponse(request *ConvNode, content string, gen *idGen) *ConvNode {
	if request == nil || request.Kind != TreeRowRequest {
		return nil
	}
	if gen == nil {
		gen = &idGen{}
	}
	resp := &ConvNode{
		ID:      gen.next(request.Session),
		Kind:    TreeRowResponse,
		Session: request.Session,
		Content: content,
		Parent:  request,
	}
	request.Children = append(request.Children, resp)
	return resp
}

// FindNode looks up an ID in the tree.
func FindNode(root *ConvNode, id string) *ConvNode {
	if root == nil || id == "" {
		return nil
	}
	if root.ID == id {
		return root
	}
	for _, c := range root.Children {
		if n := FindNode(c, id); n != nil {
			return n
		}
	}
	return nil
}

// SubmitParent returns where a new request should attach given the selected node.
// Session → session; Response → that response; Request → request's parent (sibling fork).
func SubmitParent(selected *ConvNode) *ConvNode {
	if selected == nil {
		return nil
	}
	switch selected.Kind {
	case TreeRowSession, TreeRowResponse:
		return selected
	case TreeRowRequest:
		if selected.Parent != nil {
			return selected.Parent
		}
		return selected
	default:
		return selected
	}
}

// FlattenConvTree produces visible rows with tree-drawing labels.
// expanded[id]==false collapses children; missing key defaults to true for
// the current session's nodes and false for other sessions' deep nodes —
// pass explicit map; if nil, all expanded.
func FlattenConvTree(root *ConvNode, currentKey string, titleWidth int, expanded map[string]bool) []SessionTreeRow {
	if root == nil {
		return nil
	}
	if titleWidth < 8 {
		titleWidth = 8
	}
	var out []SessionTreeRow
	var walk func(n *ConvNode, depth int, prefix string, isLast bool)
	walk = func(n *ConvNode, depth int, prefix string, isLast bool) {
		exp := true
		if expanded != nil {
			if v, ok := expanded[n.ID]; ok {
				exp = v
			} else if n.Kind == TreeRowSession {
				exp = n.Session == currentKey
			}
		}
		label := nodeLabel(n, exp, titleWidth, currentKey)
		branch := ""
		if depth > 0 {
			if isLast {
				branch = "└─ "
			} else {
				branch = "├─ "
			}
		}
		out = append(out, SessionTreeRow{
			Kind:       n.Kind,
			SessionKey: n.Session,
			Label:      prefix + branch + label,
			Content:    n.Content,
			Expanded:   exp,
			Depth:      depth,
			NodeID:     n.ID,
		})
		if !exp || len(n.Children) == 0 {
			return
		}
		childPrefix := prefix
		if depth > 0 {
			if isLast {
				childPrefix += "   "
			} else {
				childPrefix += "│  "
			}
		}
		for i, c := range n.Children {
			walk(c, depth+1, childPrefix, i == len(n.Children)-1)
		}
	}
	walk(root, 0, "", true)
	return out
}

func nodeLabel(n *ConvNode, exp bool, titleWidth int, currentKey string) string {
	twist := "▶"
	if exp && len(n.Children) > 0 {
		twist = "▼"
	}
	if len(n.Children) == 0 {
		twist = "·"
	}
	switch n.Kind {
	case TreeRowSession:
		mark := " "
		if n.Session == currentKey {
			mark = "●"
		}
		title := TruncateTitle(n.Content, titleWidth)
		if title == "" {
			title = "(empty)"
		}
		return fmt.Sprintf("%s %s %s", twist, mark, title)
	case TreeRowRequest:
		return fmt.Sprintf("%s ↑ %s", twist, TruncateTitle(n.Content, titleWidth-2))
	case TreeRowResponse:
		return fmt.Sprintf("%s ↓ %s", twist, TruncateTitle(n.Content, titleWidth-2))
	default:
		return TruncateTitle(n.Content, titleWidth)
	}
}

// FormatTreeRowLabel returns the pre-formatted label (connectors already in Label).
func FormatTreeRowLabel(row SessionTreeRow) string {
	return row.Label
}
