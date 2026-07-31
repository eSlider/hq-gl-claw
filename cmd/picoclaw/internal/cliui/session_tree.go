package cliui

import (
	"fmt"
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
	Content    string // full text for request/response; empty for session
	Expanded   bool   // session nodes only
	Depth      int
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

// BuildSessionTreeRows builds a session → request/response tree.
// expanded[key]==true shows children; current session defaults expanded when map is nil.
func BuildSessionTreeRows(
	src SessionLister,
	currentKey string,
	titleWidth int,
	expanded map[string]bool,
) []SessionTreeRow {
	if titleWidth < 8 {
		titleWidth = 8
	}
	items := BuildSessionItems(src, currentKey, titleWidth)
	out := make([]SessionTreeRow, 0, len(items)*3)
	for _, it := range items {
		exp := it.Key == currentKey
		if expanded != nil {
			if v, ok := expanded[it.Key]; ok {
				exp = v
			}
		}
		mark := " "
		if it.Key == currentKey {
			mark = "●"
		}
		twist := "▶"
		if exp {
			twist = "▼"
		}
		out = append(out, SessionTreeRow{
			Kind:       TreeRowSession,
			SessionKey: it.Key,
			Label:      fmt.Sprintf("%s %s %s", twist, mark, it.Title),
			Expanded:   exp,
			Depth:      0,
		})
		if !exp || src == nil {
			continue
		}
		turns := PairTurns(src.GetHistory(it.Key))
		// Show newest turns last (chronological).
		for _, turn := range turns {
			if turn.Request != "" {
				out = append(out, SessionTreeRow{
					Kind:       TreeRowRequest,
					SessionKey: it.Key,
					Label:      "↑ " + TruncateTitle(turn.Request, titleWidth-2),
					Content:    turn.Request,
					Depth:      1,
				})
			}
			if turn.Response != "" {
				out = append(out, SessionTreeRow{
					Kind:       TreeRowResponse,
					SessionKey: it.Key,
					Label:      "↓ " + TruncateTitle(turn.Response, titleWidth-2),
					Content:    turn.Response,
					Depth:      1,
				})
			}
		}
	}
	return out
}

// FormatTreeRowLabel indents a tree row for list display.
func FormatTreeRowLabel(row SessionTreeRow) string {
	indent := strings.Repeat("  ", row.Depth)
	return indent + row.Label
}
