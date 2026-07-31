package cliui

import "testing"

func TestEmojiProgress_Cycles(t *testing.T) {
	a := EmojiProgress(0)
	b := EmojiProgress(1)
	if a == "" || b == "" {
		t.Fatal("empty")
	}
	if a == b {
		t.Fatalf("expected different frames: %q vs %q", a, b)
	}
	// wraps
	n := len(thinkingEmojis)
	if EmojiProgress(n) != EmojiProgress(0) {
		t.Fatal("should wrap")
	}
	if !containsEmoji(a) {
		t.Fatalf("expected emoji in %q", a)
	}
}

func containsEmoji(s string) bool {
	for _, e := range thinkingEmojis {
		if len(s) >= len(e) && s[:len(e)] == e {
			return true
		}
	}
	return false
}
