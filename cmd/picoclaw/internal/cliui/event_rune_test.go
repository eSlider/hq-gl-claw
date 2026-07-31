package cliui

import "testing"

func TestEventRune_ASCII(t *testing.T) {
	r, ok := eventRune("a")
	if !ok || r != 'a' {
		t.Fatalf("got %q ok=%v", r, ok)
	}
}

func TestEventRune_Cyrillic(t *testing.T) {
	r, ok := eventRune("я")
	if !ok || r != 'я' {
		t.Fatalf("got %q ok=%v (byteLen=%d)", r, ok, len("я"))
	}
	r, ok = eventRune("Ж")
	if !ok || r != 'Ж' {
		t.Fatalf("got %q ok=%v", r, ok)
	}
}

func TestEventRune_RejectsKeys(t *testing.T) {
	for _, id := range []string{"", "<Enter>", "<C-c>", "<M-a>", "ab"} {
		if _, ok := eventRune(id); ok {
			t.Fatalf("expected reject %q", id)
		}
	}
}
