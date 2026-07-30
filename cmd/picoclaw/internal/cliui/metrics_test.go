package cliui

import (
	"strings"
	"testing"
	"time"
)

func TestFormatTurnStatus_StreamingAndDone(t *testing.T) {
	m := TurnMetrics{
		PromptTokens:     1200,
		CompletionTokens: 80,
		PromptExact:      true,
		CompletionExact:  true,
		Elapsed:          2 * time.Second,
		Streaming:        true,
	}
	got := FormatTurnStatus(m)
	for _, want := range []string{"↑1200", "↓80", "2.0s", "tps"} {
		if !strings.Contains(got, want) {
			t.Fatalf("streaming status %q missing %q", got, want)
		}
	}
	if !strings.Contains(got, "⏳") {
		t.Fatalf("expected streaming marker: %q", got)
	}

	m.Streaming = false
	got = FormatTurnStatus(m)
	if !strings.Contains(got, "✓") {
		t.Fatalf("expected done marker: %q", got)
	}
	// 80 tok / 2s = 40 tps
	if !strings.Contains(got, "40.0") {
		t.Fatalf("expected 40.0 tps: %q", got)
	}
}

func TestFormatTurnStatus_EstimatedMarked(t *testing.T) {
	m := TurnMetrics{
		PromptTokens:     10,
		CompletionTokens: 20,
		PromptExact:      false,
		CompletionExact:  false,
		Elapsed:          time.Second,
	}
	got := FormatTurnStatus(m)
	if !strings.Contains(got, "↑~10") || !strings.Contains(got, "↓~20") {
		t.Fatalf("expected ~ markers: %q", got)
	}
}

func TestFormatStatusBar_IncludesSessionTotals(t *testing.T) {
	last := TurnMetrics{
		PromptTokens: 100, CompletionTokens: 50,
		PromptExact: true, CompletionExact: true,
		Elapsed: 1500 * time.Millisecond,
	}
	sess := SessionMetrics{Turns: 3, PromptTokens: 500, CompletionTokens: 200}
	got := FormatStatusBar(last, sess, "input")
	for _, want := range []string{"↑100", "↓50", "1.5s", "Σ", "↑500", "↓200", "3 turns", "input"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status bar %q missing %q", got, want)
		}
	}
}

func TestSessionMetrics_AddTurn(t *testing.T) {
	var s SessionMetrics
	s.AddTurn(TurnMetrics{PromptTokens: 10, CompletionTokens: 5})
	s.AddTurn(TurnMetrics{PromptTokens: 20, CompletionTokens: 7})
	if s.Turns != 2 || s.PromptTokens != 30 || s.CompletionTokens != 12 {
		t.Fatalf("session=%+v", s)
	}
}

func TestPaneLayoutIncludesStatusBar(t *testing.T) {
	p := NewPaneSession(80, 24)
	stats, content, input, status := p.Slots4()
	if stats != 1 || input != 1 || status != 1 {
		t.Fatalf("chrome stats=%d input=%d status=%d", stats, input, status)
	}
	if content != 21 {
		t.Fatalf("content=%d want 21", content)
	}
	p.SetStatusBar("✓ ↑10 ↓5 · 1.0s · 5.0 tps")
	out := p.Render()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 24 {
		t.Fatalf("lines=%d want 24", len(lines))
	}
	if !strings.Contains(lines[len(lines)-1], "↑10") {
		t.Fatalf("bottom status missing metrics: %q", lines[len(lines)-1])
	}
}
