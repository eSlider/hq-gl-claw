package cliui

import (
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
//
// Icon rules (no duplicates):
//   - Session: "● title" (or "▶ ● title" when collapsed with children)
//   - Request / response: "├─↑ text" / "└─↓ text" — kind glyph once; "▶" only if collapsed
//
// expanded[id]==false collapses children. Missing keys default to expanded for
// the current session and collapsed for other sessions' roots.
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
		exp := n.Session == currentKey
		if expanded != nil {
			if v, ok := expanded[n.ID]; ok {
				exp = v
			} else if n.Kind == TreeRowSession {
				if v, ok := expanded[n.Session]; ok {
					exp = v
				} else {
					exp = n.Session == currentKey
				}
			}
		}
		hasKids := len(n.Children) > 0
		label := formatNodeRow(n, prefix, depth, isLast, exp, hasKids, titleWidth, currentKey)
		out = append(out, SessionTreeRow{
			Kind:       n.Kind,
			SessionKey: n.Session,
			Label:      label,
			Content:    n.Content,
			Expanded:   exp,
			Depth:      depth,
			NodeID:     n.ID,
		})
		if !exp || !hasKids {
			return
		}
		childPrefix := prefix
		if depth > 0 {
			if isLast {
				childPrefix += "  "
			} else {
				childPrefix += "│ "
			}
		}
		for i, c := range n.Children {
			walk(c, depth+1, childPrefix, i == len(n.Children)-1)
		}
	}
	walk(root, 0, "", true)
	return out
}

// formatNodeRow builds one list row; truncates to titleWidth runes.
func formatNodeRow(
	n *ConvNode,
	prefix string,
	depth int,
	isLast, exp, hasKids bool,
	titleWidth int,
	currentKey string,
) string {
	var b strings.Builder
	b.WriteString(prefix)
	if depth > 0 {
		if isLast {
			b.WriteString("└")
		} else {
			b.WriteString("├")
		}
		// Collapsed non-leaf: show ▶ in the connector slot; else ─.
		if hasKids && !exp {
			b.WriteString("▶")
		} else {
			b.WriteString("─")
		}
	} else if hasKids && !exp {
		b.WriteString("▶ ")
	}
	switch n.Kind {
	case TreeRowSession:
		mark := "○"
		if n.Session == currentKey {
			mark = "●"
		}
		b.WriteString(mark)
		b.WriteByte(' ')
		b.WriteString(TruncateTitle(n.Content, maxInt(4, titleWidth-runeWidthApprox(b.String()))))
	case TreeRowRequest:
		b.WriteString("↑")
		b.WriteByte(' ')
		b.WriteString(TruncateTitle(n.Content, maxInt(4, titleWidth-runeWidthApprox(b.String()))))
	case TreeRowResponse:
		b.WriteString("↓")
		b.WriteByte(' ')
		b.WriteString(TruncateTitle(n.Content, maxInt(4, titleWidth-runeWidthApprox(b.String()))))
	default:
		b.WriteString(TruncateTitle(n.Content, titleWidth))
	}
	return b.String()
}

func runeWidthApprox(s string) int {
	return len([]rune(s))
}

// DeepestTip returns the rightmost deepest node (follow last child).
func DeepestTip(root *ConvNode) *ConvNode {
	if root == nil {
		return nil
	}
	n := root
	for len(n.Children) > 0 {
		n = n.Children[len(n.Children)-1]
	}
	return n
}

// ExpandAncestors marks node and all parents expanded.
func ExpandAncestors(expanded map[string]bool, node *ConvNode) {
	if expanded == nil || node == nil {
		return
	}
	for n := node; n != nil; n = n.Parent {
		expanded[n.ID] = true
		if n.Kind == TreeRowSession {
			expanded[n.Session] = true
		}
	}
}

// FormatTreeRowLabel returns the pre-formatted label (connectors already in Label).
func FormatTreeRowLabel(row SessionTreeRow) string {
	return row.Label
}

// ReconcileConvTree appends any trailing history turns not already represented
// in the tree (keeps existing branches; extends the deepest tip spine).
func ReconcileConvTree(root *ConvNode, msgs []ChatMessage, gen *idGen) {
	if root == nil {
		return
	}
	if gen == nil {
		gen = &idGen{}
	}
	existing := PathMessages(DeepestTip(root))
	if len(msgs) <= len(existing) {
		return
	}
	// Rebuild spine from full history only when tree is a pure spine with no branches.
	if !hasBranching(root) {
		fresh := BuildConvTreeFromHistory(root.Session, root.Content, msgs, gen)
		root.Children = fresh.Children
		for _, c := range root.Children {
			c.Parent = root
		}
		return
	}
	extra := msgs[len(existing):]
	tip := DeepestTip(root)
	// Attach remaining pairs under tip (if tip is response/session) or tip.Parent.
	attachAt := tip
	if tip.Kind == TreeRowRequest {
		attachAt = tip.Parent
		if attachAt == nil {
			attachAt = root
		}
	}
	for _, turn := range PairTurns(extra) {
		if turn.Request != "" {
			req := AttachRequest(attachAt, turn.Request, gen)
			attachAt = req
		}
		if turn.Response != "" && attachAt != nil && attachAt.Kind == TreeRowRequest {
			resp := AttachResponse(attachAt, turn.Response, gen)
			attachAt = resp
		}
	}
}

func hasBranching(n *ConvNode) bool {
	if n == nil {
		return false
	}
	if n.Kind == TreeRowResponse && len(n.Children) > 1 {
		return true
	}
	if n.Kind == TreeRowSession && len(n.Children) > 1 {
		return true
	}
	for _, c := range n.Children {
		if hasBranching(c) {
			return true
		}
	}
	return false
}
