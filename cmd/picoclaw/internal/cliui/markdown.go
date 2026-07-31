package cliui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// PICOCLAW_GLAMOUR=0 disables markdown rendering (plain text fallback).
const envGlamourDisable = "PICOCLAW_GLAMOUR"

var (
	mdMu       sync.Mutex
	mdRenderer *glamour.TermRenderer
	mdWidth    int
	mdStyle    string
)

func glamourEnabled() bool {
	v := strings.TrimSpace(os.Getenv(envGlamourDisable))
	if v == "0" || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") {
		return false
	}
	return true
}

func glamourStyleName() string {
	if lipgloss.ColorProfile() == termenv.Ascii {
		return "notty"
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return "notty"
	}
	if termenv.HasDarkBackground() {
		return "dark"
	}
	return "light"
}

func markdownRendererWidth(width int) (*glamour.TermRenderer, error) {
	if width < 20 {
		width = 20
	}
	style := glamourStyleName()

	mdMu.Lock()
	defer mdMu.Unlock()
	if mdRenderer != nil && mdWidth == width && mdStyle == style {
		return mdRenderer, nil
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil, err
	}
	mdRenderer = r
	mdWidth = width
	mdStyle = style
	return mdRenderer, nil
}

// RenderMarkdown renders markdown for terminal display. On failure or when
// glamour is disabled, returns the original text unchanged.
func RenderMarkdown(markdown string) string {
	return RenderMarkdownWidth(markdown, InnerWidth())
}

// RenderMarkdownWidth is RenderMarkdown with an explicit wrap width (pane column).
func RenderMarkdownWidth(markdown string, width int) string {
	if markdown == "" || !glamourEnabled() {
		return markdown
	}
	r, err := markdownRendererWidth(width)
	if err != nil {
		return markdown
	}
	out, err := r.Render(markdown)
	if err != nil {
		return markdown
	}
	return strings.TrimRight(out, "\n")
}

// PrintAgentResponse writes the agent reply with the logo prefix. Markdown is
// rendered via glamour when enabled and suitable for the current terminal.
func PrintAgentResponse(w io.Writer, logo, response string) {
	if w == nil {
		w = os.Stdout
	}
	rendered := RenderMarkdown(response)
	if rendered == response {
		fmt.Fprintf(w, "\n%s %s\n", logo, response)
		return
	}
	// Put the logo on its own line so glamour block styles stay aligned.
	fmt.Fprintf(w, "\n%s\n%s\n", logo, rendered)
}
