package cliui

import (
	"strings"
	"testing"
)

func TestPaneLayoutSlots(t *testing.T) {
	p := NewPaneSession(80, 24)
	stats, content, input := p.Slots()
	if stats != 1 || input != 1 {
		t.Fatalf("stats=%d input=%d want 1 and 1", stats, input)
	}
	if content != 21 {
		t.Fatalf("content=%d want 21", content)
	}
	p.Resize(40, 5)
	_, content, _ = p.Slots()
	if content != 2 {
		t.Fatalf("content after resize=%d want 2", content)
	}
	p.Resize(40, 2)
	_, content, _ = p.Slots()
	if content != 1 {
		t.Fatalf("min content height=%d want 1", content)
	}
}

func TestPaneFocusTabCycle(t *testing.T) {
	p := NewPaneSession(80, 24)
	p.SetSessions([]SessionItem{{Key: "cli:a", Title: "a"}}, "cli:a")
	if p.Focus() != FocusInput {
		t.Fatalf("initial focus=%v want Input", p.Focus())
	}
	p.HandleKey(KeyTab)
	if p.Focus() != FocusResult {
		t.Fatalf("after Tab focus=%v want Result", p.Focus())
	}
	p.HandleKey(KeyTab)
	if p.Focus() != FocusSessions {
		t.Fatalf("after Tab2 focus=%v want Sessions", p.Focus())
	}
	p.HandleKey(KeyShiftTab)
	if p.Focus() != FocusResult {
		t.Fatalf("after ShiftTab focus=%v want Result", p.Focus())
	}
	p.HandleKey(KeyShiftTab)
	if p.Focus() != FocusInput {
		t.Fatalf("after ShiftTab2 focus=%v want Input", p.Focus())
	}
}

func TestPaneScrollWhenResultFocused(t *testing.T) {
	p := NewPaneSession(20, 7) // content height = 4; no sessions → full width
	p.SetContent(strings.Repeat("line\n", 20))
	p.HandleKey(KeyTab) // result
	if p.Scroll() != 0 {
		t.Fatalf("scroll start=%d", p.Scroll())
	}
	p.HandleKey(KeyDown)
	p.HandleKey(KeyDown)
	if p.Scroll() != 2 {
		t.Fatalf("scroll=%d want 2", p.Scroll())
	}
	p.HandleKey(KeyUp)
	if p.Scroll() != 1 {
		t.Fatalf("scroll=%d want 1", p.Scroll())
	}
	// Back to input via ShiftTab (Tab would go to sessions if present)
	p.HandleKey(KeyShiftTab)
	before := p.Scroll()
	p.HandleKey(KeyDown)
	if p.Scroll() != before {
		t.Fatalf("scroll changed while input focused")
	}
}

func TestPaneScrollClampOnResize(t *testing.T) {
	p := NewPaneSession(20, 6)
	p.SetContent(strings.Join([]string{"a", "b", "c", "d", "e", "f", "g", "h"}, "\n"))
	p.HandleKey(KeyTab)
	for i := 0; i < 10; i++ {
		p.HandleKey(KeyDown)
	}
	maxBefore := p.MaxScroll()
	if p.Scroll() != maxBefore {
		t.Fatalf("scroll=%d max=%d", p.Scroll(), maxBefore)
	}
	p.Resize(20, 20) // more viewport → clamp down
	if p.Scroll() != p.MaxScroll() {
		t.Fatalf("after grow scroll=%d max=%d", p.Scroll(), p.MaxScroll())
	}
	if p.Scroll() != 0 {
		t.Fatalf("tall viewport should clamp to 0, got %d", p.Scroll())
	}
}

func TestPaneVimSearch(t *testing.T) {
	p := NewPaneSession(40, 5) // content height = 2
	p.SetContent("alpha\nbeta foo\ngamma\nfoo bar\nzeta")
	p.HandleKey(KeyTab) // result
	p.HandleKey(KeyRune('/'))
	if p.Focus() != FocusSearch {
		t.Fatalf("focus=%v want Search", p.Focus())
	}
	for _, r := range "foo" {
		p.HandleKey(KeyRune(r))
	}
	p.HandleKey(KeyEnter)
	if p.Focus() != FocusResult {
		t.Fatalf("after search enter focus=%v", p.Focus())
	}
	if p.MatchLine() != 1 {
		t.Fatalf("first match line=%d want 1", p.MatchLine())
	}
	if p.Scroll() != 1 {
		t.Fatalf("first match scroll=%d want 1", p.Scroll())
	}
	p.HandleKey(KeyRune('n'))
	if p.MatchLine() != 3 {
		t.Fatalf("next match line=%d want 3", p.MatchLine())
	}
	if p.Scroll() != 3 {
		t.Fatalf("next match scroll=%d want 3", p.Scroll())
	}
	p.HandleKey(KeyRune('N'))
	if p.MatchLine() != 1 {
		t.Fatalf("prev match line=%d want 1", p.MatchLine())
	}
}

func TestPaneRenderHasThreeRegions(t *testing.T) {
	p := NewPaneSession(30, 8)
	p.SetStats("10 tok · 5.0 tps")
	p.SetContent("hello world\nsecond line")
	p.SetInput("ask me")
	p.SetStatusBar("✓ ↑10 ↓5 · 1.0s · 5.0 tps")
	out := p.Render()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 8 {
		t.Fatalf("render lines=%d want 8\n%q", len(lines), out)
	}
	if !strings.Contains(lines[0], "10 tok") {
		t.Fatalf("stats line=%q", lines[0])
	}
	if !strings.Contains(lines[len(lines)-2], "ask me") {
		t.Fatalf("input line=%q", lines[len(lines)-2])
	}
	if !strings.Contains(lines[len(lines)-1], "↑10") {
		t.Fatalf("status line=%q", lines[len(lines)-1])
	}
	body := strings.Join(lines[1:len(lines)-2], "\n")
	if !strings.Contains(body, "hello") {
		t.Fatalf("content missing: %q", body)
	}
}

func TestPaneResizeRerendersWrappedContent(t *testing.T) {
	p := NewPaneSession(10, 8)
	p.SetContent("abcdefghijKLMNOP")
	narrow := p.Render()
	p.Resize(20, 8)
	wide := p.Render()
	if narrow == wide {
		t.Fatal("expected different wrap after resize")
	}
	if !strings.Contains(wide, "abcdefghijKLMNOP") && !strings.Contains(wide, "abcdefghij") {
		t.Fatalf("wide render lost content: %q", wide)
	}
}

func TestPaneCursorPosVisibleOnInput(t *testing.T) {
	p := NewPaneSession(80, 24)
	p.prompt = "You: "
	p.SetInput("hi")
	row, col, show := p.CursorPos()
	if !show {
		t.Fatal("cursor should be visible on input focus")
	}
	// input row = 2 + contentHeight(21) = 23
	if row != 23 {
		t.Fatalf("row=%d want 23", row)
	}
	// mark(1) + "You: "(5) + cursor at end(2) = 1+1+5+2 = 9
	if col != 9 {
		t.Fatalf("col=%d want 9", col)
	}
	p.HandleKey(KeyTab) // result
	_, _, show = p.CursorPos()
	if show {
		t.Fatal("cursor hidden on result")
	}
}

func TestPaneVimKeysIgnoredInInput(t *testing.T) {
	p := NewPaneSession(40, 10)
	p.SetContent("foo\nbar")
	p.HandleKey(KeyRune('/')) // typing slash into input
	if p.Focus() != FocusInput {
		t.Fatal("slash should not leave input")
	}
	if !strings.Contains(p.Input(), "/") {
		t.Fatalf("input=%q", p.Input())
	}
}
