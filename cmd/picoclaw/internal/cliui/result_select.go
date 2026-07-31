package cliui

import (
	"strings"
	"unicode/utf8"

	ui "github.com/metaspartan/gotui/v5"
)

// textSel is a character-range selection inside the result pane lines.
type textSel struct {
	active   bool
	dragging bool
	aLine    int
	aCol     int
	bLine    int
	bCol     int
}

func (s textSel) normalized() (l0, c0, l1, c1 int) {
	l0, c0, l1, c1 = s.aLine, s.aCol, s.bLine, s.bCol
	if l1 < l0 || (l1 == l0 && c1 < c0) {
		return l1, c1, l0, c0
	}
	return l0, c0, l1, c1
}

func (s textSel) empty() bool {
	return !s.active || (s.aLine == s.bLine && s.aCol == s.bCol)
}

// ExtractSelection returns the plain text covered by the selection.
func ExtractSelection(lines []string, l0, c0, l1, c1 int) string {
	if len(lines) == 0 {
		return ""
	}
	if l0 < 0 {
		l0 = 0
	}
	if l1 >= len(lines) {
		l1 = len(lines) - 1
	}
	if l1 < l0 {
		return ""
	}
	var b strings.Builder
	for i := l0; i <= l1; i++ {
		plain := []rune(stripGotuiMarkup(lines[i]))
		start, end := 0, len(plain)
		if i == l0 {
			start = clampRune(c0, len(plain))
		}
		if i == l1 {
			end = clampRune(c1, len(plain))
		}
		if start > end {
			start, end = end, start
		}
		if i > l0 {
			b.WriteByte('\n')
		}
		b.WriteString(string(plain[start:end]))
	}
	return b.String()
}

// ApplySelectionHighlight wraps the selected plain span in a reverse-video style.
func ApplySelectionHighlight(lines []string, l0, c0, l1, c1 int) []string {
	if len(lines) == 0 || l1 < l0 || (l1 == l0 && c1 <= c0) {
		return lines
	}
	out := make([]string, len(lines))
	copy(out, lines)
	for i := l0; i <= l1 && i < len(out); i++ {
		plain := []rune(stripGotuiMarkup(out[i]))
		start, end := 0, len(plain)
		if i == l0 {
			start = clampRune(c0, len(plain))
		}
		if i == l1 {
			end = clampRune(c1, len(plain))
		}
		if start > end {
			start, end = end, start
		}
		if start == end {
			out[i] = string(plain)
			continue
		}
		var b strings.Builder
		b.WriteString(escapeGotuiText(string(plain[:start])))
		b.WriteString("[")
		b.WriteString(escapeGotuiText(string(plain[start:end])))
		b.WriteString("](fg:black,bg:white)")
		b.WriteString(escapeGotuiText(string(plain[end:])))
		out[i] = b.String()
	}
	return out
}

func clampRune(i, n int) int {
	if i < 0 {
		return 0
	}
	if i > n {
		return n
	}
	return i
}

// escapeGotuiText keeps '[' / ']' from being parsed as style markup.
func escapeGotuiText(s string) string {
	if !strings.ContainsAny(s, "[]") {
		return s
	}
	return strings.NewReplacer("[", "⟮", "]", "⟯").Replace(s)
}

// RuneIndexAtVisualCol maps a visual column to a rune index in plain text.
func RuneIndexAtVisualCol(plain string, visualCol int) int {
	if visualCol <= 0 {
		return 0
	}
	col := 0
	i := 0
	for _, r := range plain {
		w := 1
		if r == '\t' {
			w = 4
		} else if utf8.RuneLen(r) > 1 {
			// wide glyphs roughly 2; keep simple width-1 for TUI lines
			w = 1
		}
		if col+w > visualCol {
			return i
		}
		col += w
		i++
	}
	return i
}

// CopyToClipboard writes text to the system clipboard (OSC 52 via tcell).
func CopyToClipboard(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if text == "" {
		return
	}
	if ui.Screen != nil {
		ui.Screen.SetClipboard([]byte(text))
	}
}
