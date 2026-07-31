package cliui

import (
	"strings"
	"testing"
)

func TestFormatSessionTranscript_Blocks(t *testing.T) {
	msgs := []ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi\nthere"},
		{Role: "user", Content: "next"},
		{Role: "assistant", Content: "ok"},
	}
	text, blocks := FormatSessionTranscript(msgs)
	if !strings.Contains(text, "↑ request") || !strings.Contains(text, "↓ response") {
		t.Fatalf("missing markers: %q", text)
	}
	if len(blocks) != 4 {
		t.Fatalf("blocks=%d want 4", len(blocks))
	}
	if blocks[0].Kind != TreeRowRequest || blocks[0].Content != "hello" {
		t.Fatalf("block0=%+v", blocks[0])
	}
	if blocks[1].Kind != TreeRowResponse || blocks[1].LineEnd <= blocks[1].LineStart {
		t.Fatalf("block1=%+v", blocks[1])
	}
	bl, ok := FindTranscriptBlock(blocks, TreeRowRequest, "next")
	if !ok || bl.Content != "next" {
		t.Fatalf("find next: ok=%v bl=%+v", ok, bl)
	}
}

func TestApplyThinHighlight_NotRounded(t *testing.T) {
	lines := []string{"a", "b", "c", "d"}
	got := ApplyThinHighlight(lines, 1, 2, 20)
	joined := strings.Join(got, "\n")
	for _, ch := range []string{"╭", "╮", "╰", "╯"} {
		if strings.Contains(joined, ch) {
			t.Fatalf("rounded corner %q present: %q", ch, joined)
		}
	}
	if !strings.Contains(joined, "┌") || !strings.Contains(joined, "└") {
		t.Fatalf("expected thin corners: %q", joined)
	}
	if !strings.Contains(joined, "│ b") || !strings.Contains(joined, "│ c") {
		t.Fatalf("expected boxed lines: %q", joined)
	}
	// Outer lines preserved outside the box.
	if got[0] != "a" || got[len(got)-1] != "d" {
		t.Fatalf("got=%q", got)
	}
}

func TestScrollToHighlightOffset(t *testing.T) {
	if got := ScrollToHighlightOffset(5, 100, 10); got != 4 {
		t.Fatalf("got %d", got)
	}
	if got := ScrollToHighlightOffset(0, 100, 10); got != 0 {
		t.Fatalf("got %d", got)
	}
}
