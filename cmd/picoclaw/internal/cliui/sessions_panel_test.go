package cliui

import (
	"testing"
)

func TestTruncateTitle(t *testing.T) {
	if got := TruncateTitle("hello", 10); got != "hello" {
		t.Fatalf("got %q", got)
	}
	got := TruncateTitle("abcdefghijklmnopqrstuvwxyz", 8)
	if got != "abcdefgh…" {
		t.Fatalf("got %q", got)
	}
}

func TestLastUserRequestTitle(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "ok"},
		{Role: "user", Content: "second question please"},
		{Role: "assistant", Content: "sure"},
	}
	got := LastUserRequestTitle(msgs, 40)
	if got != "second question please" {
		t.Fatalf("got %q", got)
	}
	got = LastUserRequestTitle(msgs, 6)
	if got != "second…" {
		t.Fatalf("truncate got %q", got)
	}
	if got := LastUserRequestTitle(nil, 10); got != "(empty)" {
		t.Fatalf("empty=%q", got)
	}
}

type fakeLister struct {
	keys map[string][]ChatMessage
	order []string
}

func (f *fakeLister) ListSessions() []string { return f.order }
func (f *fakeLister) GetHistory(key string) []ChatMessage { return f.keys[key] }

func TestBuildSessionItems(t *testing.T) {
	src := &fakeLister{
		order: []string{"cli:a", "cli:b"},
		keys: map[string][]ChatMessage{
			"cli:a": {{Role: "user", Content: "alpha ask"}},
			"cli:b": {{Role: "user", Content: "beta ask"}},
		},
	}
	items := BuildSessionItems(src, "cli:a", 40)
	if len(items) < 2 {
		t.Fatalf("items=%d", len(items))
	}
	found := false
	for _, it := range items {
		if it.Key == "cli:a" && it.Title == "alpha ask" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing current session title: %+v", items)
	}
}
