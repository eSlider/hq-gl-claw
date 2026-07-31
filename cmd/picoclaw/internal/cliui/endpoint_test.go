package cliui

import (
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestShortAPIHost(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:9988/v1":  "127.0.0.1:9988",
		"https://api.openai.com/v1": "api.openai.com",
		"127.0.0.1:8765/v1":         "127.0.0.1:8765",
		"":                          "",
	}
	for in, want := range cases {
		if got := ShortAPIHost(in); got != want {
			t.Fatalf("ShortAPIHost(%q)=%q want %q", in, got, want)
		}
	}
}

func TestFormatEndpoint(t *testing.T) {
	got := FormatEndpoint(EndpointInfo{ModelName: "bonsai", APIBase: "http://127.0.0.1:9988/v1"})
	if got != "bonsai · 127.0.0.1:9988" {
		t.Fatalf("got %q", got)
	}
}

func TestNextModelName(t *testing.T) {
	cfg := &config.Config{
		ModelList: config.SecureModelList{
			{ModelName: "a", Model: "a", Enabled: true},
			{ModelName: "b", Model: "b", Enabled: true},
			{ModelName: "c", Model: "c", Enabled: false},
		},
	}
	if got := NextModelName(cfg, "a"); got != "b" {
		t.Fatalf("a→%q", got)
	}
	if got := NextModelName(cfg, "b"); got != "a" {
		t.Fatalf("b wraps →%q", got)
	}
}

func TestEnsureBonsaiLocal(t *testing.T) {
	cfg := &config.Config{}
	EnsureBonsaiLocal(cfg, true)
	if cfg.Agents.Defaults.ModelName != "bonsai" {
		t.Fatalf("default=%q", cfg.Agents.Defaults.ModelName)
	}
	found := false
	for _, m := range cfg.ModelList {
		if m.ModelName == "bonsai" && strings.Contains(m.APIBase, "9988") && m.Enabled {
			found = true
		}
	}
	if !found {
		t.Fatalf("bonsai entry missing: %+v", cfg.ModelList)
	}
	// Idempotent
	EnsureBonsaiLocal(cfg, false)
	n := 0
	for _, m := range cfg.ModelList {
		if m.ModelName == "bonsai" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("duplicate bonsai entries: %d", n)
	}
}

func TestFormatStatusBar_IncludesEndpoint(t *testing.T) {
	got := FormatStatusBar(TurnMetrics{}, SessionMetrics{}, "input", "bonsai · 127.0.0.1:9988")
	if !strings.Contains(got, "bonsai · 127.0.0.1:9988") {
		t.Fatalf("missing endpoint: %q", got)
	}
}
