package cliui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui/mdansi"
)

// TranscriptBlock is one request or response region in a session transcript.
type TranscriptBlock struct {
	Kind    TreeRowKind
	Content string
	// LineStart/LineEnd are inclusive indices into the plain (pre-highlight) lines.
	LineStart int
	LineEnd   int
}

// FormatSessionTranscript builds a navigable ↑/↓ transcript and block map.
// Assistant responses are rendered with mdansi gotui markup when enabled.
func FormatSessionTranscript(msgs []ChatMessage) (text string, blocks []TranscriptBlock) {
	return FormatSessionTranscriptWidth(msgs, 80)
}

// FormatSessionTranscriptWidth is FormatSessionTranscript with an explicit wrap width.
func FormatSessionTranscriptWidth(msgs []ChatMessage, width int) (text string, blocks []TranscriptBlock) {
	if width < 20 {
		width = 20
	}
	var lines []string
	add := func(s string) int {
		lines = append(lines, s)
		return len(lines) - 1
	}
	for _, turn := range PairTurns(msgs) {
		if turn.Request != "" {
			if len(lines) > 0 {
				add("")
			}
			start := add("↑ request")
			for _, ln := range strings.Split(turn.Request, "\n") {
				add(ln)
			}
			blocks = append(blocks, TranscriptBlock{
				Kind:      TreeRowRequest,
				Content:   turn.Request,
				LineStart: start,
				LineEnd:   len(lines) - 1,
			})
		}
		if turn.Response != "" {
			if len(lines) > 0 {
				add("")
			}
			start := add("↓ response")
			body := turn.Response
			if markdownEnabled() {
				body = mdansi.RenderGotui(turn.Response, width)
			}
			for _, ln := range strings.Split(body, "\n") {
				add(ln)
			}
			blocks = append(blocks, TranscriptBlock{
				Kind:      TreeRowResponse,
				Content:   turn.Response,
				LineStart: start,
				LineEnd:   len(lines) - 1,
			})
		}
	}
	return strings.Join(lines, "\n"), blocks
}

// FindTranscriptBlock returns the block matching kind+content, or false.
func FindTranscriptBlock(blocks []TranscriptBlock, kind TreeRowKind, content string) (TranscriptBlock, bool) {
	content = strings.TrimSpace(content)
	for _, bl := range blocks {
		if bl.Kind == kind && strings.TrimSpace(bl.Content) == content {
			return bl, true
		}
	}
	return TranscriptBlock{}, false
}

// ApplyThinHighlight wraps lines[start:end] (inclusive) in a thin (non-rounded) box.
// width is the target pane inner width for horizontal rules.
func ApplyThinHighlight(lines []string, start, end, width int) []string {
	n := len(lines)
	if n == 0 || start < 0 || end < start || start >= n {
		return lines
	}
	if end >= n {
		end = n - 1
	}
	if width < 8 {
		width = 8
	}
	inner := width - 4 // "│ " + content + " │"
	if inner < 4 {
		inner = 4
	}
	top := "┌" + strings.Repeat("─", inner+2) + "┐"
	bot := "└" + strings.Repeat("─", inner+2) + "┘"
	out := make([]string, 0, n+(end-start+1)+2)
	out = append(out, lines[:start]...)
	out = append(out, top)
	for i := start; i <= end; i++ {
		out = append(out, padBoxLine(lines[i], inner))
	}
	out = append(out, bot)
	out = append(out, lines[end+1:]...)
	return out
}

func padBoxLine(s string, inner int) string {
	plain := stripGotuiMarkup(s)
	rlen := utf8.RuneCountInString(plain)
	if rlen > inner {
		runes := []rune(plain)
		plain = string(runes[:inner-1]) + "…"
		rlen = inner
	}
	pad := inner - rlen
	if pad < 0 {
		pad = 0
	}
	return fmt.Sprintf("│ %s%s │", plain, strings.Repeat(" ", pad))
}

// stripGotuiMarkup removes [text](style) wrappers for width measurement / box content.
func stripGotuiMarkup(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '[' {
			end := -1
			for j := i + 1; j < len(runes); j++ {
				if runes[j] == ']' {
					end = j
					break
				}
			}
			if end > 0 && end+1 < len(runes) && runes[end+1] == '(' {
				closeParen := -1
				for j := end + 2; j < len(runes); j++ {
					if runes[j] == ')' {
						closeParen = j
						break
					}
				}
				if closeParen > 0 {
					b.WriteString(string(runes[i+1 : end]))
					i = closeParen
					continue
				}
			}
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

// ScrollToHighlightOffset places start near the top of the viewport.
func ScrollToHighlightOffset(start, total, page int) int {
	if page < 1 {
		page = 1
	}
	off := start - 1
	if off < 0 {
		off = 0
	}
	return ClampOffset(off, total, page)
}
