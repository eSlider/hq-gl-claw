package cliui

import (
	"strings"
	"testing"
)

func TestHelpShortcuts_HasCoreBindings(t *testing.T) {
	for _, want := range []string{
		"Ctrl+H",
		"Tab",
		"Ctrl+N",
		"Ctrl+J",
		"Enter",
		"sessions",
	} {
		if !strings.Contains(HelpShortcuts, want) {
			t.Fatalf("help missing %q", want)
		}
	}
}

func TestHelpOverlaySize_FitsTerminal(t *testing.T) {
	w, h := HelpOverlaySize(80, 24)
	if w > 80 || h > 24 {
		t.Fatalf("size %dx%d exceeds 80x24", w, h)
	}
	if w < 20 || h < 8 {
		t.Fatalf("size too small: %dx%d", w, h)
	}
	r := CenterRect(80, 24, w, h)
	if r.X < 0 || r.Y < 0 || r.X+r.W > 80 || r.Y+r.H > 24 {
		t.Fatalf("center rect out of bounds: %+v", r)
	}
}

func TestCenterRect_OddEven(t *testing.T) {
	r := CenterRect(10, 10, 4, 4)
	if r.X != 3 || r.Y != 3 || r.W != 4 || r.H != 4 {
		t.Fatalf("got %+v", r)
	}
}
