package cliui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui/mdansi"
)

// PICOCLAW_MARKDOWN=0 disables markdown rendering (plain text fallback).
// PICOCLAW_GLAMOUR remains accepted as a deprecated alias for one release.
const (
	envMarkdownDisable = "PICOCLAW_MARKDOWN"
	envGlamourDisable  = "PICOCLAW_GLAMOUR" // deprecated alias
)

func markdownEnabled() bool {
	for _, key := range []string{envMarkdownDisable, envGlamourDisable} {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "0" || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") {
			return false
		}
	}
	return true
}

// RenderMarkdown renders markdown for terminal display. On failure or when
// markdown styling is disabled, returns the original text unchanged.
func RenderMarkdown(markdown string) string {
	return RenderMarkdownWidth(markdown, InnerWidth())
}

// RenderMarkdownWidth is RenderMarkdown with an explicit wrap width (pane column).
func RenderMarkdownWidth(markdown string, width int) string {
	if markdown == "" || !markdownEnabled() {
		return markdown
	}
	return mdansi.Render(markdown, width)
}

// PrintAgentResponse writes the agent reply with the logo prefix. Markdown is
// rendered via mdansi when enabled.
func PrintAgentResponse(w io.Writer, logo, response string) {
	if w == nil {
		w = os.Stdout
	}
	rendered := RenderMarkdown(response)
	if rendered == response {
		fmt.Fprintf(w, "\n%s %s\n", logo, response)
		return
	}
	// Put the logo on its own line so block styles stay aligned.
	fmt.Fprintf(w, "\n%s\n%s\n", logo, rendered)
}
