package cliui

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// SessionItem is one row in the right-hand sessions panel.
type SessionItem struct {
	Key   string
	Title string // last user request, truncated
}

// ChatMessage is a minimal history entry for title extraction.
type ChatMessage struct {
	Role    string
	Content string
}

// TruncateTitle trims whitespace and truncates to maxRunes with an ellipsis.
func TruncateTitle(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}

// LastUserRequestTitle returns the last user message as a truncated title.
func LastUserRequestTitle(msgs []ChatMessage, maxRunes int) string {
	title := ""
	for _, m := range msgs {
		if strings.EqualFold(m.Role, "user") {
			c := strings.TrimSpace(m.Content)
			if c != "" {
				title = c
			}
		}
	}
	if title == "" {
		return "(empty)"
	}
	return TruncateTitle(title, maxRunes)
}

// LastAssistantContent returns the last assistant text, or "".
func LastAssistantContent(msgs []ChatMessage) string {
	out := ""
	for _, m := range msgs {
		if strings.EqualFold(m.Role, "assistant") {
			c := strings.TrimSpace(m.Content)
			if c != "" {
				out = c
			}
		}
	}
	return out
}

// SessionLister abstracts session store listing for the pane UI.
type SessionLister interface {
	ListSessions() []string
	GetHistory(key string) []ChatMessage
}

// BuildSessionItems lists cli:* sessions (plus currentKey), titles from last user request.
// retainKeys keeps sessions visible in the tree even if the store has not listed
// them yet (e.g. after Ctrl+N before the previous key is re-queried).
//
// Order is stable: newest cli:* keys first (by key). The active session is NOT
// moved — only the ●/○ glyph marks it. Reordering on select made the list jump
// and hid previous sessions under the fold. Non-cli channel sessions are omitted
// so startup does not load every workspace transcript.
func BuildSessionItems(src SessionLister, currentKey string, titleWidth int, retainKeys ...string) []SessionItem {
	if titleWidth < 8 {
		titleWidth = 8
	}
	if src == nil {
		return []SessionItem{{Key: currentKey, Title: "(empty)"}}
	}
	keys := src.ListSessions()
	cliKeys := make([]string, 0, len(keys)+len(retainKeys)+1)
	seen := map[string]struct{}{}
	add := func(k string) {
		if k == "" {
			return
		}
		if _, ok := seen[k]; ok {
			return
		}
		// Interactive agent pane lists cli:* only (plus explicit currentKey).
		if !strings.HasPrefix(k, "cli:") && k != currentKey {
			return
		}
		seen[k] = struct{}{}
		cliKeys = append(cliKeys, k)
	}
	for _, k := range keys {
		add(k)
	}
	for _, k := range retainKeys {
		add(k)
	}
	add(currentKey)
	// Newest-looking keys first (cli:<unixnano> sorts lexicographically by time).
	sort.SliceStable(cliKeys, func(i, j int) bool { return cliKeys[i] > cliKeys[j] })
	items := make([]SessionItem, 0, len(cliKeys))
	for _, k := range cliKeys {
		title := LastUserRequestTitle(src.GetHistory(k), titleWidth)
		items = append(items, SessionItem{Key: k, Title: title})
	}
	if len(items) == 0 {
		items = append(items, SessionItem{Key: currentKey, Title: "(empty)"})
	}
	return items
}

// NewSessionKey returns a fresh cli session key.
func NewSessionKey() string {
	return fmt.Sprintf("cli:%d", time.Now().UnixNano())
}
