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
	keys  map[string][]ChatMessage
	order []string
}

func (f *fakeLister) ListSessions() []string              { return f.order }
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

func TestBuildSessionItems_StableOrderNoActiveJump(t *testing.T) {
	src := &fakeLister{
		order: []string{"cli:100", "cli:200", "cli:150"},
		keys: map[string][]ChatMessage{
			"cli:100": {{Role: "user", Content: "old"}},
			"cli:150": {{Role: "user", Content: "mid"}},
			"cli:200": {{Role: "user", Content: "new"}},
		},
	}
	// Active is the oldest — must NOT jump to index 0.
	items := BuildSessionItems(src, "cli:100", 40)
	if len(items) != 3 {
		t.Fatalf("items=%d %+v", len(items), items)
	}
	if items[0].Key != "cli:200" {
		t.Fatalf("newest should stay first, got %q", items[0].Key)
	}
	if items[2].Key != "cli:100" {
		t.Fatalf("active oldest should stay last, got %+v", items)
	}
	// Selecting a different current key must not reshuffle relative order.
	items2 := BuildSessionItems(src, "cli:150", 40)
	for i := range items {
		if items[i].Key != items2[i].Key {
			t.Fatalf("order changed on select: %v vs %v", keysOf(items), keysOf(items2))
		}
	}
}

func keysOf(items []SessionItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Key
	}
	return out
}

func TestBuildSessionItems_RetainKeys(t *testing.T) {
	src := &fakeLister{
		order: []string{"cli:new"},
		keys: map[string][]ChatMessage{
			"cli:new": {},
			"cli:old": {{Role: "user", Content: "keep me"}},
		},
	}
	items := BuildSessionItems(src, "cli:new", 40, "cli:old")
	found := false
	for _, it := range items {
		if it.Key == "cli:old" && it.Title == "keep me" {
			found = true
		}
	}
	if !found {
		t.Fatalf("retain key missing: %+v", items)
	}
}

func TestBuildSessionItems_SkipsNonCLI(t *testing.T) {
	src := &fakeLister{
		order: []string{"sk_v1_channel", "cli:a", "agent:main:x"},
		keys: map[string][]ChatMessage{
			"sk_v1_channel": {{Role: "user", Content: "channel"}},
			"cli:a":         {{Role: "user", Content: "cli"}},
			"agent:main:x":  {{Role: "user", Content: "agent"}},
		},
	}
	items := BuildSessionItems(src, "cli:a", 40)
	if len(items) != 1 || items[0].Key != "cli:a" {
		t.Fatalf("want only cli:a, got %+v", items)
	}
}
