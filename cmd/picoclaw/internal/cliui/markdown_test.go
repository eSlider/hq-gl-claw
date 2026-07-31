package cliui

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderMarkdown_DisabledEnv(t *testing.T) {
	t.Setenv(envMarkdownDisable, "0")
	in := "# Title\n\n**bold**"
	if got := RenderMarkdown(in); got != in {
		t.Fatalf("disabled markdown should pass through, got %q", got)
	}
}

func TestRenderMarkdown_DisabledGlamourAlias(t *testing.T) {
	t.Setenv(envMarkdownDisable, "")
	t.Setenv(envGlamourDisable, "0")
	in := "# Title"
	if got := RenderMarkdown(in); got != in {
		t.Fatalf("PICOCLAW_GLAMOUR=0 alias should disable, got %q", got)
	}
}

func TestRenderMarkdown_RendersHeading(t *testing.T) {
	t.Setenv(envMarkdownDisable, "1")
	in := "# Hello\n\nWorld"
	got := RenderMarkdown(in)
	if got == in {
		t.Fatalf("expected rendered markdown to differ from input")
	}
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
		t.Fatalf("rendered output missing content: %q", got)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI styling, got %q", got)
	}
}

func TestPrintAgentResponse_PlainFallback(t *testing.T) {
	t.Setenv(envMarkdownDisable, "0")
	var buf bytes.Buffer
	PrintAgentResponse(&buf, "🦞", "plain reply")
	got := buf.String()
	if !strings.Contains(got, "🦞 plain reply") {
		t.Fatalf("unexpected plain output: %q", got)
	}
}

func TestPrintAgentResponse_Mdansi(t *testing.T) {
	t.Setenv(envMarkdownDisable, "1")
	var buf bytes.Buffer
	PrintAgentResponse(&buf, "🦞", "# Hi\n\n- one")
	got := buf.String()
	if !strings.Contains(got, "🦞") {
		t.Fatalf("missing logo: %q", got)
	}
	if !strings.Contains(got, "Hi") {
		t.Fatalf("missing heading text: %q", got)
	}
}
