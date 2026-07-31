package cliui

import (
	"strings"
	"testing"
)

func TestViewWindow_Basic(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}
	got := ViewWindow(lines, 1, 2)
	if got != "b\nc" {
		t.Fatalf("got %q", got)
	}
}

func TestViewWindow_Clamp(t *testing.T) {
	lines := []string{"a", "b"}
	got := ViewWindow(lines, 10, 5)
	if got != "a\nb" {
		t.Fatalf("overscroll should clamp, got %q", got)
	}
}

func TestViewWindow_ANSILines(t *testing.T) {
	lines := []string{"\x1b[1mA\x1b[0m", "B", "C"}
	got := ViewWindow(lines, 0, 2)
	if !strings.Contains(got, "\x1b[1mA") || !strings.Contains(got, "B") {
		t.Fatalf("got %q", got)
	}
}

func TestScrollOffset_Page(t *testing.T) {
	off := ClampOffset(0, 100, 10)
	off = PageDown(off, 100, 10)
	if off != 10 {
		t.Fatalf("page down got %d", off)
	}
	off = PageUp(off, 10)
	if off != 0 {
		t.Fatalf("page up got %d", off)
	}
}
