package cliui

import (
	"strings"
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
		t.Fatalf("truncated got %q", got)
	}
	if got := LastUserRequestTitle(nil, 10); got != "(empty)" {
		t.Fatalf("empty=%q", got)
	}
}

func TestPaneTabCycleIncludesSessions(t *testing.T) {
	p := NewPaneSession(80, 24)
	p.SetSessions([]SessionItem{{Key: "cli:a", Title: "a"}}, "cli:a")
	if p.Focus() != FocusInput {
		t.Fatal("start input")
	}
	p.HandleKey(KeyTab)
	if p.Focus() != FocusResult {
		t.Fatalf("tab1=%v", p.Focus())
	}
	p.HandleKey(KeyTab)
	if p.Focus() != FocusSessions {
		t.Fatalf("tab2=%v", p.Focus())
	}
	p.HandleKey(KeyTab)
	if p.Focus() != FocusInput {
		t.Fatalf("tab3=%v", p.Focus())
	}
	p.HandleKey(KeyShiftTab)
	if p.Focus() != FocusSessions {
		t.Fatalf("shift1=%v", p.Focus())
	}
	p.HandleKey(KeyShiftTab)
	if p.Focus() != FocusResult {
		t.Fatalf("shift2=%v", p.Focus())
	}
	p.HandleKey(KeyShiftTab)
	if p.Focus() != FocusInput {
		t.Fatalf("shift3=%v", p.Focus())
	}
}

func TestPaneSessionsVimNavAndSelect(t *testing.T) {
	p := NewPaneSession(80, 20)
	p.SetSessions([]SessionItem{
		{Key: "cli:a", Title: "alpha ask"},
		{Key: "cli:b", Title: "beta ask"},
		{Key: "cli:c", Title: "gamma ask"},
	}, "cli:a")
	p.HandleKey(KeyTab) // result
	p.HandleKey(KeyTab) // sessions
	if p.Focus() != FocusSessions {
		t.Fatal(p.Focus())
	}
	if p.SessionCursor() != 0 {
		t.Fatalf("cursor=%d", p.SessionCursor())
	}
	p.HandleKey(KeyRune('j'))
	p.HandleKey(KeyRune('j'))
	if p.SessionCursor() != 2 {
		t.Fatalf("cursor=%d want 2", p.SessionCursor())
	}
	p.HandleKey(KeyRune('k'))
	if p.SessionCursor() != 1 {
		t.Fatalf("cursor=%d want 1", p.SessionCursor())
	}
	msg, action := p.HandleKey(KeyEnter)
	if action != KeyActionSwitchSession || msg != "cli:b" {
		t.Fatalf("action=%v msg=%q", action, msg)
	}
	if p.ActiveSessionKey() != "cli:b" {
		t.Fatalf("active=%q", p.ActiveSessionKey())
	}
}

func TestPaneRenderHasSessionsColumn(t *testing.T) {
	p := NewPaneSession(60, 12)
	p.SetSessions([]SessionItem{
		{Key: "cli:default", Title: "what is go"},
		{Key: "cli:old", Title: "previous chat about rust"},
	}, "cli:default")
	p.SetContent("answer body")
	p.SetInput("next")
	out := p.Render()
	if !strings.Contains(out, "what is go") {
		t.Fatalf("missing current title:\n%s", out)
	}
	if !strings.Contains(out, "previous") {
		t.Fatalf("missing previous:\n%s", out)
	}
	if !strings.Contains(out, "│") && !strings.Contains(out, "┃") {
		// either content focus bar or column divider
		t.Fatalf("expected column divider:\n%s", out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 12 {
		t.Fatalf("lines=%d", len(lines))
	}
}
