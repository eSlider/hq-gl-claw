package cliui

import (
	"strings"
)

// TreeRowKind identifies a row in the sessions tree panel.
type TreeRowKind int

const (
	TreeRowSession TreeRowKind = iota
	TreeRowRequest
	TreeRowResponse
)

// SessionTreeRow is one visible/logical row in the sessions tree.
type SessionTreeRow struct {
	Kind       TreeRowKind
	SessionKey string
	Label      string
	Content    string // full text for request/response; empty for session title source
	Expanded   bool
	Depth      int
	NodeID     string // ConvNode.ID for selection / submit anchoring
}

// TurnPair is one user request + optional assistant response.
type TurnPair struct {
	Request  string
	Response string
}

// PairTurns walks history into request/response pairs (orphan assistant → empty request).
func PairTurns(msgs []ChatMessage) []TurnPair {
	var out []TurnPair
	var cur *TurnPair
	for _, m := range msgs {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		c := strings.TrimSpace(m.Content)
		if c == "" {
			continue
		}
		switch role {
		case "user":
			if cur != nil {
				out = append(out, *cur)
			}
			cur = &TurnPair{Request: c}
		case "assistant":
			if cur == nil {
				cur = &TurnPair{}
			}
			cur.Response = c
			out = append(out, *cur)
			cur = nil
		}
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

// BuildSessionTreeRows builds nested session trees for all listed sessions.
// expanded is keyed by ConvNode.ID (and session key for backward-compatible
// session collapse). retainKeys are always kept (see BuildSessionItems).
func BuildSessionTreeRows(
	src SessionLister,
	currentKey string,
	titleWidth int,
	forest map[string]*ConvNode,
	expanded map[string]bool,
	gen *idGen,
	retainKeys ...string,
) ([]SessionTreeRow, map[string]*ConvNode) {
	if titleWidth < 8 {
		titleWidth = 8
	}
	if forest == nil {
		forest = map[string]*ConvNode{}
	}
	if gen == nil {
		gen = &idGen{}
	}
	items := BuildSessionItems(src, currentKey, titleWidth, retainKeys...)
	out := make([]SessionTreeRow, 0, len(items)*4)
	for _, it := range items {
		root, ok := forest[it.Key]
		if !ok || root == nil {
			var msgs []ChatMessage
			if src != nil {
				msgs = src.GetHistory(it.Key)
			}
			root = BuildConvTreeFromHistory(it.Key, it.Title, msgs, gen)
			forest[it.Key] = root
		} else {
			root.Content = it.Title
			if src != nil {
				ReconcileConvTree(root, src.GetHistory(it.Key), gen)
			}
		}
		// Honor session-key collapse in expanded map.
		sessionCollapsed := false
		if expanded != nil {
			if v, ok := expanded[it.Key]; ok {
				expanded[root.ID] = v
				sessionCollapsed = !v
			}
			if v, ok := expanded[root.ID]; ok && !v {
				sessionCollapsed = true
			}
		}
		// Keep the active spine visible unless the session is collapsed.
		if it.Key == currentKey && expanded != nil && !sessionCollapsed {
			ExpandAncestors(expanded, DeepestTip(root))
		}
		out = append(out, FlattenConvTree(root, currentKey, titleWidth, expanded)...)
	}
	return out, forest
}
