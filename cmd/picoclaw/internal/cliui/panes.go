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
	FocusSessions
	FocusSearch
)

// KeyAction is the high-level outcome of HandleKey.
type KeyAction int

const (
	KeyActionNone KeyAction = iota
	KeyActionSubmit
	KeyActionSwitchSession
	KeyActionNewSession
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

	prompt    string
	statusBar string // bottom overall metrics line

	sessions      []SessionItem
	sessionKey    string
	sessionCur    int
	sessionScroll int
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

func (p *PaneSession) SessionCursor() int {
	if p == nil {
		return 0
	}
	return p.sessionCur
}

func (p *PaneSession) ActiveSessionKey() string {
	if p == nil {
		return ""
	}
	return p.sessionKey
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

func (p *PaneSession) StatusBar() string {
	if p == nil {
		return ""
	}
	return p.statusBar
}

func (p *PaneSession) Width() int  { return p.width }
func (p *PaneSession) Height() int { return p.height }

// Slots returns stats, content, and input row counts (legacy 3-chrome without status).
// Prefer Slots4.
func (p *PaneSession) Slots() (stats, content, input int) {
	stats, content, input, _ = p.Slots4()
	return stats, content, input
}

// Slots4 returns top stats, content, input, and bottom status row counts.
func (p *PaneSession) Slots4() (stats, content, input, status int) {
	stats, input, status = 1, 1, 1
	content = p.height - 3
	if content < 1 {
		content = 1
	}
	return stats, content, input, status
}

// ContentHeight is the viewport height for the result pane.
func (p *PaneSession) ContentHeight() int {
	h := p.height - 3
	if h < 1 {
		return 1
	}
	return h
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
	if height < 4 {
		height = 4
	}
	p.width = width
	p.height = height
	p.rewrap()
	p.clampScroll()
}

// SetStats sets the one-line statistics header (progress / focus hints).
func (p *PaneSession) SetStats(s string) {
	p.stats = strings.ReplaceAll(s, "\n", " ")
}

// SetStatusBar sets the bottom overall metrics line.
func (p *PaneSession) SetStatusBar(s string) {
	p.statusBar = strings.ReplaceAll(s, "\n", " ")
}

// SetSessions replaces the right-hand session list; currentKey is marked active.
func (p *PaneSession) SetSessions(items []SessionItem, currentKey string) {
	p.sessions = append([]SessionItem(nil), items...)
	p.sessionKey = currentKey
	p.sessionCur = 0
	for i, it := range p.sessions {
		if it.Key == currentKey {
			p.sessionCur = i
			break
		}
	}
	p.clampSessionCursor()
}

// UpdateSessionTitle sets/inserts the title for key (after a turn).
func (p *PaneSession) UpdateSessionTitle(key, title string) {
	if key == "" {
		return
	}
	title = TruncateTitle(title, p.sessionTitleWidth())
	for i := range p.sessions {
		if p.sessions[i].Key == key {
			p.sessions[i].Title = title
			return
		}
	}
	p.sessions = append([]SessionItem{{Key: key, Title: title}}, p.sessions...)
	p.sessionKey = key
	p.sessionCur = 0
}

func (p *PaneSession) sessionTitleWidth() int {
	w := p.sideWidth() - 2
	if w < 8 {
		w = 8
	}
	return w
}

func (p *PaneSession) sideWidth() int {
	if len(p.sessions) == 0 {
		return 0
	}
	w := p.width / 4
	if w < 18 {
		w = 18
	}
	if w > 36 {
		w = 36
	}
	if p.width-w < 28 {
		w = p.width - 28
	}
	if w < 14 {
		if p.width < 42 {
			return 0
		}
		return 14
	}
	return w
}

func (p *PaneSession) leftWidth() int {
	sw := p.sideWidth()
	if sw == 0 {
		return p.width
	}
	return p.width - sw - 1 // divider column
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
	wrapWidth := p.leftWidth() - 1 // content prefix column
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

// HandleKey processes a key. Payload depends on KeyAction (submit text or session key).
func (p *PaneSession) HandleKey(k Key) (payload string, action KeyAction) {
	switch p.focus {
	case FocusSearch:
		p.handleSearchKey(k)
		return "", KeyActionNone
	case FocusResult:
		return p.handleResultKey(k)
	case FocusSessions:
		return p.handleSessionsKey(k)
	default:
		return p.handleInputKey(k)
	}
}

func (p *PaneSession) cycleFocus(forward bool) {
	order := []Focus{FocusInput, FocusResult, FocusSessions}
	if p.sideWidth() == 0 {
		order = []Focus{FocusInput, FocusResult}
	}
	idx := 0
	for i, f := range order {
		if f == p.focus {
			idx = i
			break
		}
	}
	if forward {
		idx = (idx + 1) % len(order)
	} else {
		idx = (idx - 1 + len(order)) % len(order)
	}
	p.focus = order[idx]
}

func (p *PaneSession) handleInputKey(k Key) (string, KeyAction) {
	switch k {
	case KeyTab:
		p.cycleFocus(true)
		return "", KeyActionNone
	case KeyShiftTab:
		p.cycleFocus(false)
		return "", KeyActionNone
	case KeyEnter:
		msg := strings.TrimSpace(p.input)
		p.input = ""
		p.inputCursor = 0
		if msg == "" {
			return "", KeyActionNone
		}
		return msg, KeyActionSubmit
	case KeyBackspace:
		p.backspaceInput()
		return "", KeyActionNone
	case KeyLeft:
		if p.inputCursor > 0 {
			p.inputCursor--
		}
		return "", KeyActionNone
	case KeyRight:
		if p.inputCursor < utf8.RuneCountInString(p.input) {
			p.inputCursor++
		}
		return "", KeyActionNone
	case KeyEsc:
		return "", KeyActionNone
	}
	if r, ok := k.rune(); ok && r >= 32 {
		p.insertInput(r)
	}
	return "", KeyActionNone
}

func (p *PaneSession) handleResultKey(k Key) (string, KeyAction) {
	vh := p.ContentHeight()
	switch k {
	case KeyTab:
		p.cycleFocus(true)
		return "", KeyActionNone
	case KeyShiftTab:
		p.cycleFocus(false)
		return "", KeyActionNone
	case KeyDown, KeyRune('j'):
		p.scroll++
	case KeyUp, KeyRune('k'):
		p.scroll--
	case KeyPageDown:
		p.scroll += vh
	case KeyPageUp:
		p.scroll -= vh
	case KeyRune('d'):
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
		return "", KeyActionNone
	case KeyRune('n'):
		p.jumpSearch(1)
		return "", KeyActionNone
	case KeyRune('N'):
		p.jumpSearch(-1)
		return "", KeyActionNone
	case KeyEsc:
		p.searchQuery = ""
		p.searchMatches = nil
		p.searchIdx = 0
		p.matchLine = -1
		return "", KeyActionNone
	}
	p.clampScroll()
	return "", KeyActionNone
}

func (p *PaneSession) handleSessionsKey(k Key) (string, KeyAction) {
	vh := p.sessionViewportHeight()
	switch k {
	case KeyTab:
		p.cycleFocus(true)
		return "", KeyActionNone
	case KeyShiftTab:
		p.cycleFocus(false)
		return "", KeyActionNone
	case KeyDown, KeyRune('j'):
		p.sessionCur++
		p.clampSessionCursor()
		return "", KeyActionNone
	case KeyUp, KeyRune('k'):
		p.sessionCur--
		p.clampSessionCursor()
		return "", KeyActionNone
	case KeyPageDown:
		p.sessionCur += vh
		p.clampSessionCursor()
		return "", KeyActionNone
	case KeyPageUp:
		p.sessionCur -= vh
		p.clampSessionCursor()
		return "", KeyActionNone
	case KeyHome, KeyRune('g'):
		p.sessionCur = 0
		p.clampSessionCursor()
		return "", KeyActionNone
	case KeyEnd, KeyRune('G'):
		p.sessionCur = len(p.sessions) - 1
		p.clampSessionCursor()
		return "", KeyActionNone
	case KeyEnter:
		if p.sessionCur >= 0 && p.sessionCur < len(p.sessions) {
			key := p.sessions[p.sessionCur].Key
			p.sessionKey = key
			return key, KeyActionSwitchSession
		}
		return "", KeyActionNone
	case KeyRune('n'):
		return NewSessionKey(), KeyActionNewSession
	case KeyEsc:
		p.focus = FocusInput
		return "", KeyActionNone
	}
	return "", KeyActionNone
}

func (p *PaneSession) sessionViewportHeight() int {
	// sessions column spans content + input rows
	h := p.height - 2
	if h < 1 {
		return 1
	}
	return h
}

func (p *PaneSession) clampSessionCursor() {
	if len(p.sessions) == 0 {
		p.sessionCur = 0
		p.sessionScroll = 0
		return
	}
	if p.sessionCur < 0 {
		p.sessionCur = 0
	}
	if p.sessionCur >= len(p.sessions) {
		p.sessionCur = len(p.sessions) - 1
	}
	vh := p.sessionViewportHeight() - 1 // header row
	if vh < 1 {
		vh = 1
	}
	if p.sessionCur < p.sessionScroll {
		p.sessionScroll = p.sessionCur
	}
	if p.sessionCur >= p.sessionScroll+vh {
		p.sessionScroll = p.sessionCur - vh + 1
	}
	if p.sessionScroll < 0 {
		p.sessionScroll = 0
	}
}

func (p *PaneSession) handleSearchKey(k Key) {
	switch k {
	case KeyEsc:
		p.searchQuery = ""
		p.searchMatches = nil
		p.focus = FocusResult
		return
	case KeyTab:
		p.searchQuery = ""
		p.searchMatches = nil
		p.focus = FocusResult
		p.cycleFocus(true)
		return
	case KeyShiftTab:
		p.searchQuery = ""
		p.searchMatches = nil
		p.focus = FocusResult
		p.cycleFocus(false)
		return
	case KeyEnter:
		p.recomputeSearch()
		p.focus = FocusResult
		if len(p.searchMatches) > 0 {
			p.searchIdx = 0
			p.scrollToMatch(0)
		}
		return
	case KeyBackspace:
		if p.searchQuery == "" {
			return
		}
		runes := []rune(p.searchQuery)
		p.searchQuery = string(runes[:len(runes)-1])
		return
	}
	if r, ok := k.rune(); ok && r >= 32 {
		p.searchQuery += string(r)
	}
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
	contentH := p.ContentHeight()
	lw := p.leftWidth()
	sw := p.sideWidth()
	var b strings.Builder

	// Top: live progress / focus (full width)
	b.WriteString(padTrim(p.decorateStats(), p.width))
	b.WriteByte('\n')

	sideLines := p.renderSessionsColumn(contentH + 1) // content + input rows

	// Content viewport + side
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
		bodyW := lw - 1
		if bodyW < 1 {
			bodyW = 1
		}
		left := prefix + padTrim(line, bodyW)
		left = padTrim(left, lw)
		if sw > 0 {
			b.WriteString(left)
			b.WriteString("┃")
			b.WriteString(sideLines[row])
		} else {
			b.WriteString(padTrim(left, p.width))
		}
		b.WriteByte('\n')
	}

	// Input / search + side continuation
	inputLine := padTrim(p.decorateInput(), lw)
	if sw > 0 {
		b.WriteString(inputLine)
		b.WriteString("┃")
		b.WriteString(sideLines[contentH])
	} else {
		b.WriteString(padTrim(inputLine, p.width))
	}
	b.WriteByte('\n')

	// Bottom status bar (full width)
	b.WriteString(padTrim(p.decorateStatus(), p.width))
	return b.String()
}

func (p *PaneSession) renderSessionsColumn(rows int) []string {
	sw := p.sideWidth()
	out := make([]string, rows)
	if sw == 0 {
		for i := range out {
			out[i] = ""
		}
		return out
	}
	header := "sessions"
	if p.focus == FocusSessions {
		header = "▸ sessions"
	}
	out[0] = padTrim(header, sw)
	if rows <= 1 {
		return out
	}
	for i := 1; i < rows; i++ {
		idx := p.sessionScroll + (i - 1)
		if idx < 0 || idx >= len(p.sessions) {
			out[i] = padTrim("", sw)
			continue
		}
		it := p.sessions[idx]
		mark := " "
		if it.Key == p.sessionKey {
			mark = "●"
		}
		if idx == p.sessionCur && p.focus == FocusSessions {
			mark = "▸"
			if it.Key == p.sessionKey {
				mark = "●"
			}
		}
		title := TruncateTitle(it.Title, sw-2)
		out[i] = padTrim(mark+title, sw)
	}
	return out
}

func (p *PaneSession) decorateStats() string {
	focus := "input"
	switch p.focus {
	case FocusResult:
		focus = "result"
	case FocusSessions:
		focus = "sessions"
	case FocusSearch:
		focus = "search"
	}
	base := p.stats
	if base == "" {
		base = "ready"
	}
	return fmt.Sprintf("%s  · %s · Tab panes  /search", base, focus)
}

func (p *PaneSession) decorateInput() string {
	if p.focus == FocusSearch {
		return "/" + p.searchQuery
	}
	mark := " "
	if p.focus == FocusInput {
		mark = ">"
	}
	return mark + p.prompt + p.input
}

// CursorPos returns 1-based ANSI row/col for the text cursor and whether it should be visible.
func (p *PaneSession) CursorPos() (row, col int, visible bool) {
	if p == nil {
		return 1, 1, false
	}
	// Layout rows (1-based): 1=stats, 2..1+contentH=content, 2+contentH=input, 3+contentH=status
	inputRow := 2 + p.ContentHeight()
	switch p.focus {
	case FocusInput:
		col = 1 + 1 + utf8.RuneCountInString(p.prompt) + p.inputCursor // mark + prompt + caret
		lw := p.leftWidth()
		if col > lw {
			col = lw
		}
		if col < 1 {
			col = 1
		}
		return inputRow, col, true
	case FocusSearch:
		col = 1 + 1 + utf8.RuneCountInString(p.searchQuery) // '/' + query (caret at end)
		lw := p.leftWidth()
		if col > lw {
			col = lw
		}
		if col < 1 {
			col = 1
		}
		return inputRow, col, true
	default:
		return inputRow, 1, false
	}
}

func (p *PaneSession) decorateStatus() string {
	if p.statusBar != "" {
		return p.statusBar
	}
	return "↑in ↓out · elapsed · tps · Tab focus"
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
