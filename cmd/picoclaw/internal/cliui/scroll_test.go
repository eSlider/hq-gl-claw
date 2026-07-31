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

func TestScrollbarThumbRange_HiddenWhenFits(t *testing.T) {
	_, _, ok := ScrollbarThumbRange(5, 0, 10)
	if ok {
		t.Fatal("expected no scrollbar when content fits")
	}
}

func TestScrollbarThumbRange_MovesWithOffset(t *testing.T) {
	s0, e0, ok := ScrollbarThumbRange(100, 0, 10)
	if !ok || s0 != 0 {
		t.Fatalf("top thumb: %d-%d ok=%v", s0, e0, ok)
	}
	s1, e1, ok := ScrollbarThumbRange(100, 90, 10)
	if !ok || e1 != 10 {
		t.Fatalf("bottom thumb: %d-%d ok=%v", s1, e1, ok)
	}
	if s1 <= s0 {
		t.Fatalf("thumb should move down: top=%d bottom=%d", s0, s1)
	}
}

func TestApplyScrollbar_PaintsThumb(t *testing.T) {
	window := []string{"a", "b", "c", "d"}
	got := ApplyScrollbar(window, 20, 0, 4, 10)
	if len(got) != 4 {
		t.Fatalf("len=%d", len(got))
	}
	if !strings.Contains(got[0], glyphScrollThumb) {
		t.Fatalf("expected thumb on first rows: %q", got[0])
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, glyphScrollTrack) {
		t.Fatalf("expected track: %q", joined)
	}
}

func TestFitLineWidth_Pads(t *testing.T) {
	got := FitLineWidth("hi", 5)
	if got != "hi   " {
		t.Fatalf("got %q", got)
	}
}
