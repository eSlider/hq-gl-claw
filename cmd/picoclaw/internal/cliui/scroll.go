package cliui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// ViewWindow returns lines[offset:offset+height] joined by newlines.
func ViewWindow(lines []string, offset, height int) string {
	return strings.Join(ViewWindowLines(lines, offset, height), "\n")
}

// ViewWindowLines returns the visible slice of lines for a viewport.
func ViewWindowLines(lines []string, offset, height int) []string {
	if height < 1 {
		height = 1
	}
	n := len(lines)
	if n == 0 {
		return nil
	}
	maxOff := n - height
	if maxOff < 0 {
		maxOff = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOff {
		offset = maxOff
	}
	end := offset + height
	if end > n {
		end = n
	}
	return lines[offset:end]
}

// ScrollbarThumbRange returns the inclusive viewport row range [start, end)
// occupied by the scrollbar thumb. ok is false when content fits the page.
func ScrollbarThumbRange(total, offset, page int) (start, end int, ok bool) {
	if page < 1 || total <= page {
		return 0, 0, false
	}
	thumb := page * page / total
	if thumb < 1 {
		thumb = 1
	}
	if thumb > page {
		thumb = page
	}
	maxOff := total - page
	travel := page - thumb
	if maxOff < 1 || travel < 1 {
		return 0, thumb, true
	}
	offset = ClampOffset(offset, total, page)
	start = (offset * travel) / maxOff
	end = start + thumb
	if end > page {
		end = page
		start = end - thumb
	}
	return start, end, true
}

// FitLineWidth pads or truncates a gotui markup line to contentW cells.
func FitLineWidth(s string, contentW int) string {
	if contentW < 1 {
		contentW = 1
	}
	plain := stripGotuiMarkup(s)
	w := runewidth.StringWidth(plain)
	if w == contentW {
		if plain == s {
			return s
		}
		// Markup may expand; keep original when visible width already fits.
		return s
	}
	if w < contentW {
		return s + strings.Repeat(" ", contentW-w)
	}
	return escapeGotuiText(runewidth.Truncate(plain, contentW, ""))
}

// ApplyScrollbar paints a right-edge track/thumb into each viewport line.
// width is the full inner pane width; one column is reserved for the bar.
// When content fits, lines are returned unchanged (no gutter).
func ApplyScrollbar(window []string, total, offset, page, width int) []string {
	thumbStart, thumbEnd, show := ScrollbarThumbRange(total, offset, page)
	if !show || width < 2 {
		return window
	}
	contentW := width - 1
	out := make([]string, len(window))
	for i, line := range window {
		fitted := FitLineWidth(line, contentW)
		glyph := glyphScrollTrack
		style := "fg:white"
		if i >= thumbStart && i < thumbEnd {
			glyph = glyphScrollThumb
			style = "fg:cyan"
		}
		out[i] = fitted + "[" + glyph + "](" + style + ")"
	}
	// Pad short viewports so the track fills the pane height.
	for i := len(out); i < page; i++ {
		pad := strings.Repeat(" ", contentW)
		glyph := glyphScrollTrack
		style := "fg:white"
		if i >= thumbStart && i < thumbEnd {
			glyph = glyphScrollThumb
			style = "fg:cyan"
		}
		out = append(out, pad+"["+glyph+"]("+style+")")
	}
	return out
}

// ClampOffset keeps offset in [0, max(0, total-page)].
func ClampOffset(offset, total, page int) int {
	if page < 1 {
		page = 1
	}
	maxOff := total - page
	if maxOff < 0 {
		maxOff = 0
	}
	if offset < 0 {
		return 0
	}
	if offset > maxOff {
		return maxOff
	}
	return offset
}

// PageDown advances offset by page size.
func PageDown(offset, total, page int) int {
	return ClampOffset(offset+page, total, page)
}

// PageUp retreats offset by page size.
func PageUp(offset, page int) int {
	return ClampOffset(offset-page, offset+page, page)
}
