package cliui

import (
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/sipeed/picoclaw/pkg/bus"
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

func TestFormatTurnStatus_ThinkingPhaseNoZeroTPS(t *testing.T) {
	m := TurnMetrics{
		PromptTokens:    100,
		ReasoningTokens: 40,
		PromptExact:     false,
		Elapsed:         1500 * time.Millisecond,
		Streaming:       true,
	}
	got := FormatTurnStatus(m)
	if strings.Contains(got, "tps") {
		t.Fatalf("thinking phase should not show tps: %q", got)
	}
	for _, want := range []string{"thinking", "↓think~40", "⏳"} {
		if !strings.Contains(got, want) {
			t.Fatalf("thinking status %q missing %q", got, want)
		}
	}
}

func TestFormatTurnStatus_TPSExcludesTTFT(t *testing.T) {
	// 100 tokens generated in 2s after 3s think → 50 tps, not 20.
	m := TurnMetrics{
		PromptTokens:     10,
		CompletionTokens: 100,
		PromptExact:      true,
		CompletionExact:  true,
		Elapsed:          5 * time.Second,
		TTFT:             3 * time.Second,
		Streaming:        false,
	}
	if m.GenDuration() != 2*time.Second {
		t.Fatalf("GenDuration=%v", m.GenDuration())
	}
	got := FormatTurnStatus(m)
	if !strings.Contains(got, "50.0") {
		t.Fatalf("expected 50.0 tps excluding think: %q", got)
	}
	if !strings.Contains(got, "ttft") {
		t.Fatalf("expected ttft: %q", got)
	}
}

func TestPaneStreamer_ImplementsReasoningStreamer(t *testing.T) {
	var _ bus.ReasoningStreamer = NewPaneStreamer(NewAgentTUI(""))
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

func TestChromeIncludesStatusBar(t *testing.T) {
	c := ComputeChrome(80, 24, 1)
	if c.Status.H != 3 || c.Input.H != 1 {
		t.Fatalf("chrome input=%d status=%d", c.Input.H, c.Status.H)
	}
	if c.Status.W != 80 {
		t.Fatalf("status W=%d want full width 80", c.Status.W)
	}
	if c.Result.H != 20 { // 24 - 1 - 3
		t.Fatalf("result H=%d want 20", c.Result.H)
	}
	bar := FormatStatusBar(
		TurnMetrics{
			PromptTokens:     10,
			CompletionTokens: 5,
			PromptExact:      true,
			CompletionExact:  true,
			Elapsed:          time.Second,
		},
		SessionMetrics{},
		"input",
	)
	if !strings.Contains(bar, "↑10") {
		t.Fatalf("status missing metrics: %q", bar)
	}
	combined := FormatCombinedStatusBar(bar, "◐", 60)
	if !strings.Contains(combined, "↑10") || !strings.Contains(combined, "◐") {
		t.Fatalf("combined status %q", combined)
	}
	if runewidth.StringWidth(combined) > 60 {
		t.Fatalf("combined wider than width: %q", combined)
	}
}

func TestFormatCombinedStatusBar_RightAligned(t *testing.T) {
	left := "⏳ ↑10 ↓5 · 1.0s · 5.0 tps"
	right := "◐ 12.3 tps"
	got := FormatCombinedStatusBar(left, right, 50)
	if !strings.HasSuffix(got, " | ◐ 12.3 tps") {
		t.Fatalf("suffix: %q", got)
	}
	if runewidth.StringWidth(got) != 50 {
		t.Fatalf("width=%d got %q", runewidth.StringWidth(got), got)
	}
}
