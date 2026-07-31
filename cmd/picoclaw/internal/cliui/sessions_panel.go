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

// BuildSessionItems lists sessions (cli:* preferred), titles from last user request.
func BuildSessionItems(src SessionLister, currentKey string, titleWidth int) []SessionItem {
	if titleWidth < 8 {
		titleWidth = 8
	}
	if src == nil {
		return []SessionItem{{Key: currentKey, Title: "(empty)"}}
	}
	keys := src.ListSessions()
	cliKeys := make([]string, 0, len(keys))
	other := make([]string, 0)
	seen := map[string]struct{}{}
	for _, k := range keys {
		if k == "" {
			continue
		}
		seen[k] = struct{}{}
		if strings.HasPrefix(k, "cli:") {
			cliKeys = append(cliKeys, k)
		} else {
			other = append(other, k)
		}
	}
	if _, ok := seen[currentKey]; !ok && currentKey != "" {
		cliKeys = append(cliKeys, currentKey)
	}
	sort.Strings(cliKeys)
	sort.Strings(other)
	ordered := cliKeys
	if len(ordered) == 0 {
		ordered = other
	}
	items := make([]SessionItem, 0, len(ordered))
	for _, k := range ordered {
		title := LastUserRequestTitle(src.GetHistory(k), titleWidth)
		items = append(items, SessionItem{Key: k, Title: title})
	}
	if len(items) == 0 {
		items = append(items, SessionItem{Key: currentKey, Title: "(empty)"})
	}
	// Active session first, then the rest (previous) in reverse chrono by key suffix.
	return orderSessionsActiveFirst(items, currentKey)
}

func orderSessionsActiveFirst(items []SessionItem, currentKey string) []SessionItem {
	if len(items) <= 1 {
		return items
	}
	out := make([]SessionItem, 0, len(items))
	rest := make([]SessionItem, 0, len(items))
	for _, it := range items {
		if it.Key == currentKey {
			out = append(out, it)
		} else {
			rest = append(rest, it)
		}
	}
	// Newest-looking keys last in list below current → reverse sort rest.
	sort.SliceStable(rest, func(i, j int) bool { return rest[i].Key > rest[j].Key })
	return append(out, rest...)
}

// NewSessionKey returns a fresh cli session key.
func NewSessionKey() string {
	return fmt.Sprintf("cli:%d", time.Now().UnixNano())
}
