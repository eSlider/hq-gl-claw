package cliui

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestEstimateTokens(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abcd", 1},
		{"abcdefgh", 2},
		{"你好世界", 1},
		{strings.Repeat("x", 40), 10},
	}
	for _, tc := range cases {
		if got := EstimateTokens(tc.in); got != tc.want {
			t.Fatalf("EstimateTokens(%q)=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestTPS(t *testing.T) {
	if got := TPS(100, 2*time.Second); got != 50 {
		t.Fatalf("TPS= %v want 50", got)
	}
	if got := TPS(10, 0); got != 0 {
		t.Fatalf("zero duration should yield 0, got %v", got)
	}
}

func TestFormatTPSLine(t *testing.T) {
	got := FormatTPSLine(42, 28.456)
	if !strings.Contains(got, "42") || !strings.Contains(got, "tps") || !strings.Contains(got, "28.5") {
		t.Fatalf("unexpected line: %q", got)
	}
}

func TestProgressBar_Render(t *testing.T) {
	bar := NewProgressBar(20)
	s0 := bar.Render(0)
	s1 := bar.Render(1)
	if !strings.Contains(s0, "◐") {
		t.Fatalf("expected spinner: %q", s0)
	}
	if s0 == s1 {
		t.Fatalf("expected animation frame to change")
	}
	if !strings.Contains(s0, "Thinking") {
		t.Fatalf("expected Thinking label: %q", s0)
	}
}

func TestLiveDisplay_StreamsDeltas(t *testing.T) {
	var out, errBuf bytes.Buffer
	d := NewLiveDisplay(&out, &errBuf, "🦞")
	d.typeDelay = 0
	d.Start()
	time.Sleep(20 * time.Millisecond)
	if err := d.Update(context.Background(), "Hel"); err != nil {
		t.Fatal(err)
	}
	if err := d.Update(context.Background(), "Hello"); err != nil {
		t.Fatal(err)
	}
	if err := d.Finalize(context.Background(), "Hello"); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Hello") {
		t.Fatalf("missing streamed text: %q", got)
	}
	// Hel + lo should appear as Hello once in the body before tps line.
	body := strings.Split(got, "\n")
	joined := strings.Join(body, "")
	if !strings.Contains(joined, "Hello") {
		t.Fatalf("expected Hello in stream: %q", got)
	}
	if !strings.Contains(got, "tps") {
		t.Fatalf("expected tps footer: %q", got)
	}
}

func TestLiveDisplay_SetTurnUsageExact(t *testing.T) {
	var out, errBuf bytes.Buffer
	d := NewLiveDisplay(&out, &errBuf, "🦞")
	d.typeDelay = 0
	d.StartPrompt("hello world")
	d.SetTurnUsage(100, 40)
	d.Finish("done")
	got := out.String()
	if !strings.Contains(got, "↑100") || !strings.Contains(got, "↓40") {
		t.Fatalf("expected exact usage in footer: %q", got)
	}
	if strings.Contains(got, "↑~") || strings.Contains(got, "↓~") {
		t.Fatalf("exact usage should not use ~: %q", got)
	}
}

func TestStreamDelegate_ReturnsCLIStreamer(t *testing.T) {
	var out, errBuf bytes.Buffer
	d := NewLiveDisplay(&out, &errBuf, "🦞")
	del := NewStreamDelegate(d)
	s, ok := del.GetStreamer(context.Background(), "cli", "direct", "sk")
	if !ok || s == nil {
		t.Fatal("expected cli streamer")
	}
	if _, ok := del.GetStreamer(context.Background(), "telegram", "1", ""); ok {
		t.Fatal("non-cli should not match")
	}
}

func TestEnableCLIAgentStreaming(t *testing.T) {
	cfg := &config.Config{
		Channels:  config.ChannelsConfig{},
		ModelList: []*config.ModelConfig{{ModelName: "cursor-local", Model: "composer-2.5"}},
		Agents:    config.AgentsConfig{Defaults: config.AgentDefaults{ModelName: "cursor-local"}},
	}
	if err := EnableCLIAgentStreaming(cfg); err != nil {
		t.Fatal(err)
	}
	ch := cfg.Channels["cli"]
	if ch == nil {
		t.Fatal("missing cli channel")
	}
	decoded, err := ch.GetDecoded()
	if err != nil {
		t.Fatal(err)
	}
	var enabled bool
	switch s := decoded.(type) {
	case *config.WeComSettings:
		enabled = s.Streaming.Enabled
	case config.WeComSettings:
		enabled = s.Streaming.Enabled
	default:
		raw, _ := json.Marshal(decoded)
		t.Fatalf("unexpected decoded type %T: %s", decoded, raw)
	}
	if !enabled {
		t.Fatal("cli streaming not enabled")
	}
	if !cfg.ModelList[0].Streaming.Enabled {
		t.Fatal("model streaming not enabled")
	}
}
