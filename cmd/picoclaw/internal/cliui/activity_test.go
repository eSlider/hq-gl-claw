package cliui

import (
	"strings"
	"testing"
	"time"
)

func TestFormatActivityLine_Idle(t *testing.T) {
	if got := FormatActivityLine(ActivityIdle, "", time.Time{}, 0, time.Now()); got != "" {
		t.Fatalf("idle=%q", got)
	}
}

func TestFormatActivityLine_Tool(t *testing.T) {
	start := time.Unix(0, 0)
	now := start.Add(1500 * time.Millisecond)
	got := FormatActivityLine(ActivityTool, "read_file", start, 0, now)
	for _, want := range []string{"tool read_file", "1.5s", "◐"} {
		if !strings.Contains(got, want) {
			t.Fatalf("got %q missing %q", got, want)
		}
	}
}

func TestFormatActiveTools(t *testing.T) {
	if got := FormatActiveTools(map[string]int{"read_file": 1}); got != "read_file" {
		t.Fatalf("one=%q", got)
	}
	got := FormatActiveTools(map[string]int{"a": 1, "b": 1})
	if !strings.Contains(got, "+1") {
		t.Fatalf("multi=%q", got)
	}
}

func TestFormatStatusBar_IncludesActivity(t *testing.T) {
	got := FormatStatusBar(
		TurnMetrics{PromptTokens: 10, PromptExact: true, Streaming: true},
		SessionMetrics{},
		"input",
		"bonsai · local",
		"◐ tool read_file · 1.2s",
	)
	if !strings.Contains(got, "tool read_file") {
		t.Fatalf("missing activity: %q", got)
	}
	if !strings.Contains(got, "bonsai") {
		t.Fatalf("missing endpoint: %q", got)
	}
}
