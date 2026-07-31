package cliui

import (
	"strings"
	"testing"
)

func TestEmojiProgress_Cycles(t *testing.T) {
	a := EmojiProgress(0)
	b := EmojiProgress(1)
	if a == "" || b == "" {
		t.Fatal("empty")
	}
	if a == b {
		t.Fatalf("expected different frames: %q vs %q", a, b)
	}
	n := len(progressSpinners)
	if EmojiProgress(n) != EmojiProgress(0) {
		t.Fatal("should wrap")
	}
	if !strings.Contains(a, "◐") && !hasSpinner(a) {
		t.Fatalf("expected spinner in %q", a)
	}
	if !strings.Contains(a, "thinking") {
		t.Fatalf("expected thinking label in %q", a)
	}
}

func TestSpinnerFrame_Sequence(t *testing.T) {
	want := []string{"◐", "◓", "◑", "◒", "◐"}
	for i, w := range want {
		if got := SpinnerFrame(i); got != w {
			t.Fatalf("tick %d: got %q want %q", i, got, w)
		}
	}
}

func hasSpinner(s string) bool {
	for _, e := range progressSpinners {
		if strings.Contains(s, e) {
			return true
		}
	}
	return false
}
