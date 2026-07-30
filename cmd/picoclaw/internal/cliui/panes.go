package cliui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Focus identifies the active pane.
type Focus int

const (
	FocusInput Focus = iota
	FocusResult
	FocusSearch
)

// Key is a logical key event for the pane session.
type Key int

const (
	KeyTab Key = iota
	KeyShiftTab
	KeyUp
	KeyDown
	KeyPageUp
	KeyPageDown
	KeyHome
	KeyEnd
	KeyEnter
	KeyBackspace
	KeyEsc
	KeyLeft
	KeyRight
)

// keyRuneBase marks Keys that carry a Unicode code point via KeyRune(r).
const keyRuneBase Key = 1 << 16

// KeyRune builds a Key for a printable rune.
func KeyRune(r rune) Key {
	return keyRuneBase + Key(r)
}

func (k Key) rune() (rune, bool) {
	if k < keyRuneBase {
		return 0, false
	}
	return rune(k - keyRuneBase), true
}

// PaneSession is a pure pane UI state machine (no TTY I/O).
type PaneSession struct {
	width, height int
	focus         Focus

	stats   string
	content string
	lines   []string // wrapped for current width
	scroll  int

	input       string
	inputCursor int

	searchQuery   string
	searchMatches []int
	searchIdx     int
	matchLine     int // current match line (-1 if none)

	prompt string
}

// NewPaneSession creates a session with the given terminal size.
func NewPaneSession(width, height int) *PaneSession {
	p := &PaneSession{
		focus:     FocusInput,
		prompt:    "You: ",
		matchLine: -1,
	}
	p.Resize(width, height)
	return p
}

func (p *PaneSession) Focus() Focus { return p.focus }
func (p *PaneSession) Scroll() int  { return p.scroll }
func (p *PaneSession) MatchLine() int {
	if p == nil {
		return -1
	}
	return p.matchLine
}

func (p *PaneSession) Input() string {
	if p == nil {
		return ""
	}
	return p.input
}

func (p *PaneSession) Content() string {
	if p == nil {
		return ""
	}
	return p.content
}

func (p *PaneSession) Stats() string {
	if p == nil {
		return ""
	}
	return p.stats
}
func (p *PaneSession) Width() int  { return p.width }
func (p *PaneSession) Height() int { return p.height }

// Slots returns stats, content, and input row counts.
func (p *PaneSession) Slots() (stats, content, input int) {
	stats, input = 1, 1
	content = p.height - 2
	if content < 1 {
		content = 1
	}
	return stats, content, input
}

// ContentHeight is the viewport height for the result pane.
func (p *PaneSession) ContentHeight() int {
	_, c, _ := p.Slots()
	return c
}

// MaxScroll is the maximum first-line index for the viewport.
func (p *PaneSession) MaxScroll() int {
	vh := p.ContentHeight()
	limit := len(p.lines) - vh
	if limit < 0 {
		return 0
	}
	return limit
}

func (p *PaneSession) clampScroll() {
	if p.scroll < 0 {
		p.scroll = 0
	}
	if m := p.MaxScroll(); p.scroll > m {
		p.scroll = m
	}
}

// Resize updates dimensions, rewraps content, and clamps scroll.
func (p *PaneSession) Resize(width, height int) {
	if width < 8 {
		width = 8
	}
	if height < 3 {
		height = 3
	}
	p.width = width
	p.height = height
	p.rewrap()
	p.clampScroll()
}

// SetStats sets the one-line statistics header.
func (p *PaneSession) SetStats(s string) {
	p.stats = strings.ReplaceAll(s, "\n", " ")
}

// SetContent replaces result text and rewraps.
func (p *PaneSession) SetContent(s string) {
	p.content = s
	p.rewrap()
	p.clampScroll()
	if p.searchQuery != "" {
		p.recomputeSearch()
	}
}

// AppendContent appends text (streaming) and rewraps.
func (p *PaneSession) AppendContent(delta string) {
	if delta == "" {
		return
	}
	p.content += delta
	p.rewrap()
	// Follow tail while generating if already near bottom.
	if p.scroll >= p.MaxScroll()-1 {
		p.scroll = p.MaxScroll()
	}
	p.clampScroll()
}

// SetInput replaces the input buffer.
func (p *PaneSession) SetInput(s string) {
	p.input = s
	p.inputCursor = utf8.RuneCountInString(s)
}

func (p *PaneSession) rewrap() {
	wrapWidth := p.width
	if wrapWidth < 1 {
		wrapWidth = 1
	}
	p.lines = wrapText(p.content, wrapWidth)
}

func wrapText(s string, width int) []string {
	if s == "" {
		return []string{}
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		if para == "" {
			out = append(out, "")
			continue
		}
		runes := []rune(para)
		for len(runes) > 0 {
			n := width
			if n > len(runes) {
				n = len(runes)
			}
			out = append(out, string(runes[:n]))
			runes = runes[n:]
		}
	}
	return out
}

// HandleKey processes a key. When Enter submits from the input pane, it returns
// the message and true.
func (p *PaneSession) HandleKey(k Key) (submitted string, ok bool) {
	switch p.focus {
	case FocusSearch:
		return "", p.handleSearchKey(k)
	case FocusResult:
		p.handleResultKey(k)
		return "", false
	default:
		return p.handleInputKey(k)
	}
}

func (p *PaneSession) handleInputKey(k Key) (string, bool) {
	switch k {
	case KeyTab, KeyShiftTab:
		p.focus = FocusResult
		return "", false
	case KeyEnter:
		msg := strings.TrimSpace(p.input)
		p.input = ""
		p.inputCursor = 0
		if msg == "" {
			return "", false
		}
		return msg, true
	case KeyBackspace:
		p.backspaceInput()
		return "", false
	case KeyLeft:
		if p.inputCursor > 0 {
			p.inputCursor--
		}
		return "", false
	case KeyRight:
		if p.inputCursor < utf8.RuneCountInString(p.input) {
			p.inputCursor++
		}
		return "", false
	case KeyEsc:
		return "", false
	}
	if r, ok := k.rune(); ok && r >= 32 {
		p.insertInput(r)
	}
	return "", false
}

func (p *PaneSession) handleResultKey(k Key) {
	vh := p.ContentHeight()
	switch k {
	case KeyTab, KeyShiftTab:
		p.focus = FocusInput
		return
	case KeyDown, KeyRune('j'):
		p.scroll++
	case KeyUp, KeyRune('k'):
		p.scroll--
	case KeyPageDown:
		p.scroll += vh
	case KeyPageUp:
		p.scroll -= vh
	case KeyRune('d'): // half-page down (vim-ish without requiring Ctrl)
		step := vh / 2
		if step < 1 {
			step = 1
		}
		p.scroll += step
	case KeyRune('u'):
		step := vh / 2
		if step < 1 {
			step = 1
		}
		p.scroll -= step
	case KeyHome, KeyRune('g'):
		p.scroll = 0
	case KeyEnd, KeyRune('G'):
		p.scroll = p.MaxScroll()
	case KeyRune('/'):
		p.focus = FocusSearch
		p.searchQuery = ""
		return
	case KeyRune('n'):
		p.jumpSearch(1)
		return
	case KeyRune('N'):
		p.jumpSearch(-1)
		return
	case KeyEsc:
		p.searchQuery = ""
		p.searchMatches = nil
		p.searchIdx = 0
		p.matchLine = -1
		return
	}
	p.clampScroll()
}

func (p *PaneSession) handleSearchKey(k Key) bool {
	switch k {
	case KeyEsc:
		p.searchQuery = ""
		p.searchMatches = nil
		p.focus = FocusResult
		return false
	case KeyEnter:
		p.recomputeSearch()
		p.focus = FocusResult
		if len(p.searchMatches) > 0 {
			p.searchIdx = 0
			p.scrollToMatch(0)
		}
		return false
	case KeyBackspace:
		if p.searchQuery == "" {
			return false
		}
		runes := []rune(p.searchQuery)
		p.searchQuery = string(runes[:len(runes)-1])
		return false
	}
	if r, ok := k.rune(); ok && r >= 32 {
		p.searchQuery += string(r)
	}
	return false
}

func (p *PaneSession) recomputeSearch() {
	p.searchMatches = nil
	if p.searchQuery == "" {
		return
	}
	for i, line := range p.lines {
		if strings.Contains(line, p.searchQuery) {
			p.searchMatches = append(p.searchMatches, i)
		}
	}
}

func (p *PaneSession) jumpSearch(delta int) {
	if len(p.searchMatches) == 0 {
		if p.searchQuery != "" {
			p.recomputeSearch()
		}
		if len(p.searchMatches) == 0 {
			return
		}
	}
	p.searchIdx = (p.searchIdx + delta) % len(p.searchMatches)
	if p.searchIdx < 0 {
		p.searchIdx += len(p.searchMatches)
	}
	p.scrollToMatch(p.searchIdx)
}

func (p *PaneSession) scrollToMatch(idx int) {
	if idx < 0 || idx >= len(p.searchMatches) {
		p.matchLine = -1
		return
	}
	line := p.searchMatches[idx]
	p.matchLine = line
	p.scroll = line
	p.clampScroll()
}

func (p *PaneSession) insertInput(r rune) {
	runes := []rune(p.input)
	if p.inputCursor > len(runes) {
		p.inputCursor = len(runes)
	}
	out := make([]rune, 0, len(runes)+1)
	out = append(out, runes[:p.inputCursor]...)
	out = append(out, r)
	out = append(out, runes[p.inputCursor:]...)
	p.input = string(out)
	p.inputCursor++
}

func (p *PaneSession) backspaceInput() {
	runes := []rune(p.input)
	if p.inputCursor == 0 || len(runes) == 0 {
		return
	}
	i := p.inputCursor - 1
	p.input = string(append(runes[:i], runes[i+1:]...))
	p.inputCursor--
}

// Render returns a full-frame string with exactly Height lines (newline-separated).
func (p *PaneSession) Render() string {
	_, contentH, _ := p.Slots()
	var b strings.Builder

	// Stats
	b.WriteString(padTrim(p.decorateStats(), p.width))
	b.WriteByte('\n')

	// Content viewport
	start := p.scroll
	for row := 0; row < contentH; row++ {
		idx := start + row
		line := ""
		if idx >= 0 && idx < len(p.lines) {
			line = p.lines[idx]
			if p.searchQuery != "" && strings.Contains(line, p.searchQuery) {
				line = highlightMatch(line, p.searchQuery)
			}
		}
		prefix := " "
		if p.focus == FocusResult {
			prefix = "│"
		}
		// prefix takes 1 col
		bodyW := p.width - 1
		if bodyW < 1 {
			bodyW = 1
		}
		b.WriteString(prefix)
		b.WriteString(padTrim(line, bodyW))
		b.WriteByte('\n')
	}

	// Input / search
	b.WriteString(padTrim(p.decorateBottom(), p.width))
	return b.String()
}

func (p *PaneSession) decorateStats() string {
	focus := "input"
	switch p.focus {
	case FocusResult:
		focus = "result"
	case FocusSearch:
		focus = "search"
	}
	base := p.stats
	if base == "" {
		base = "ready"
	}
	return fmt.Sprintf("%s  · %s · Tab focus  / search", base, focus)
}

func (p *PaneSession) decorateBottom() string {
	if p.focus == FocusSearch {
		return "/" + p.searchQuery
	}
	mark := " "
	if p.focus == FocusInput {
		mark = ">"
	}
	return mark + p.prompt + p.input
}

func padTrim(s string, width int) string {
	runes := []rune(s)
	if len(runes) > width {
		return string(runes[:width])
	}
	if len(runes) < width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	return s
}

func highlightMatch(line, query string) string {
	if query == "" || !strings.Contains(line, query) {
		return line
	}
	// Simple markers (no ANSI) so tests stay stable; TTY can color later.
	return strings.ReplaceAll(line, query, "«"+query+"»")
}
