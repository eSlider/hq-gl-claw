package cliui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
	"golang.org/x/term"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal/cliui/mdansi"
)

// PanesEnabled reports whether the interactive TUI should be used (TTY + env).
func PanesEnabled() bool {
	if os.Getenv("PICOCLAW_PANES") == "0" {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// AgentTUI is the gotui-based interactive agent console.
type AgentTUI struct {
	prompt string

	stats    *widgets.Paragraph
	result   *widgets.Paragraph
	sessions *widgets.List
	input    *widgets.TextArea
	status   *widgets.Paragraph

	mu        sync.Mutex
	focus     Focus
	width     int
	height    int
	inputRows int

	resultLines []string
	resultOff   int
	plainBuf    string
	pretty      bool

	treeRows      []SessionTreeRow
	treeSel       int
	treeTop       int
	treeExpanded  map[string]bool
	sessionLister SessionLister
	currentKey    string

	session SessionMetrics
	last    TurnMetrics
	busy    atomic.Bool
	quit    atomic.Bool

	redrawCh chan struct{}
	eventCh  chan PaneEvent
}

// NewAgentTUI builds the interactive TUI (not yet initialized on the terminal).
func NewAgentTUI(prompt string) *AgentTUI {
	stats := widgets.NewParagraph()
	stats.Title = "stats"
	stats.Border = true
	stats.BorderRounded = true
	stats.Text = "ready"

	result := widgets.NewParagraph()
	result.Title = "result"
	result.Border = true
	result.BorderRounded = true
	result.WrapText = false

	sessions := widgets.NewList()
	sessions.Title = "sessions"
	sessions.Border = true
	sessions.BorderRounded = true
	sessions.SelectedStyle = ui.NewStyle(ui.ColorBlack, ui.ColorCyan)

	input := widgets.NewTextArea()
	input.Title = "input"
	input.Border = true
	input.BorderRounded = true
	input.ShowCursor = true
	input.Text = ""

	status := widgets.NewParagraph()
	status.Title = ""
	status.Border = true
	status.BorderRounded = true
	status.Text = "↑in ↓out · elapsed · tps · waiting for first turn"

	if prompt == "" {
		prompt = "You: "
	}

	return &AgentTUI{
		prompt:       prompt,
		stats:        stats,
		result:       result,
		sessions:     sessions,
		input:        input,
		status:       status,
		focus:        FocusInput,
		width:        80,
		height:       24,
		inputRows:    3,
		treeExpanded: map[string]bool{},
		redrawCh:     make(chan struct{}, 1),
		eventCh:      make(chan PaneEvent, 8),
	}
}

// NewPaneUI is kept as an alias for AgentTUI construction (helpers compatibility).
func NewPaneUI(prompt string) (*AgentTUI, error) {
	return NewAgentTUI(prompt), nil
}

func (t *AgentTUI) requestRedraw() {
	select {
	case t.redrawCh <- struct{}{}:
	default:
	}
}

func (t *AgentTUI) applyChromeLocked() {
	c := ComputeChrome(t.width, t.height, t.inputRows)
	setWidgetRect(t.stats, c.Stats)
	setWidgetRect(t.result, c.Result)
	setWidgetRect(t.sessions, c.Sessions)
	setWidgetRect(t.input, c.Input)
	setWidgetRect(t.status, c.Status)
	t.refreshResultViewLocked()
	t.refreshSessionsViewLocked()
	t.highlightFocusLocked()
}

func setWidgetRect(w interface {
	SetRect(x1, y1, x2, y2 int)
}, r Rect,
) {
	w.SetRect(r.X, r.Y, r.X+r.W, r.Y+r.H)
}

func (t *AgentTUI) highlightFocusLocked() {
	idle := ui.NewStyle(ui.ColorWhite)
	active := ui.NewStyle(ui.ColorCyan)
	t.stats.BorderStyle.Fg = idle.Fg
	t.result.BorderStyle.Fg = idle.Fg
	t.sessions.BorderStyle.Fg = idle.Fg
	t.input.BorderStyle.Fg = idle.Fg
	switch t.focus {
	case FocusResult:
		t.result.BorderStyle.Fg = active.Fg
	case FocusSessions:
		t.sessions.BorderStyle.Fg = active.Fg
	default:
		t.input.BorderStyle.Fg = active.Fg
	}
	t.input.ShowCursor = t.focus == FocusInput
	t.refreshStatusLocked()
}

func (t *AgentTUI) refreshStatusLocked() {
	t.status.Text = FormatStatusBar(t.last, t.session, t.focus.String())
}

func (t *AgentTUI) refreshResultViewLocked() {
	h := t.result.Inner.Dy()
	if h < 1 {
		h = t.result.Dy() - 2
	}
	if h < 1 {
		h = 1
	}
	t.resultOff = ClampOffset(t.resultOff, len(t.resultLines), h)
	t.result.Text = ViewWindow(t.resultLines, t.resultOff, h)
}

func (t *AgentTUI) setResultPlainLocked(content string) {
	t.plainBuf = content
	t.pretty = false
	t.resultLines = strings.Split(content, "\n")
	t.resultOff = maxInt(0, len(t.resultLines)-maxInt(1, t.result.Inner.Dy()))
	t.refreshResultViewLocked()
}

func (t *AgentTUI) setResultPrettyLocked(content string) {
	t.plainBuf = content
	t.pretty = true
	w := t.result.Inner.Dx()
	if w < 20 {
		w = 40
	}
	rendered := content
	if markdownEnabled() {
		rendered = mdansi.RenderGotui(content, w)
	}
	t.resultLines = strings.Split(rendered, "\n")
	t.resultOff = maxInt(0, len(t.resultLines)-maxInt(1, t.result.Inner.Dy()))
	t.refreshResultViewLocked()
}

// SyncSessions refreshes the right-hand session tree from a lister.
func (t *AgentTUI) SyncSessions(src SessionLister, currentKey string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessionLister = src
	t.currentKey = currentKey
	if _, ok := t.treeExpanded[currentKey]; !ok {
		t.treeExpanded[currentKey] = true
	}
	t.rebuildTreeLocked()
	t.requestRedraw()
}

func (t *AgentTUI) rebuildTreeLocked() {
	tw := 16
	if t.sessions.Inner.Dx() > 4 {
		tw = t.sessions.Inner.Dx() - 4
	}
	t.treeRows = BuildSessionTreeRows(t.sessionLister, t.currentKey, tw, t.treeExpanded)
	// Prefer selecting the current session row after rebuild.
	sel := 0
	for i, r := range t.treeRows {
		if r.Kind == TreeRowSession && r.SessionKey == t.currentKey {
			sel = i
			break
		}
	}
	t.treeSel = sel
	t.refreshSessionsViewLocked()
}

func (t *AgentTUI) refreshSessionsViewLocked() {
	h := maxInt(1, t.sessions.Inner.Dy())
	n := len(t.treeRows)
	t.treeTop = ClampOffset(t.treeTop, n, h)
	t.treeSel = ClampOffset(t.treeSel, n, 1)
	if n == 0 {
		t.sessions.Rows = nil
		t.sessions.SelectedRow = 0
		return
	}
	// Keep selection visible.
	if t.treeSel < t.treeTop {
		t.treeTop = t.treeSel
	}
	if t.treeSel >= t.treeTop+h {
		t.treeTop = t.treeSel - h + 1
	}
	end := t.treeTop + h
	if end > n {
		end = n
	}
	rows := make([]string, 0, end-t.treeTop)
	for i := t.treeTop; i < end; i++ {
		rows = append(rows, FormatTreeRowLabel(t.treeRows[i]))
	}
	t.sessions.Rows = rows
	t.sessions.SelectedRow = t.treeSel - t.treeTop
}

// ShowSessionContent loads text into the result pane for a switched session.
func (t *AgentTUI) ShowSessionContent(content string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if content == "" {
		t.setResultPlainLocked("")
	} else {
		t.setResultPrettyLocked(content)
	}
	t.requestRedraw()
}

func (t *AgentTUI) applyTurnMetrics(m TurnMetrics) {
	t.mu.Lock()
	defer t.mu.Unlock()
	m.Streaming = false
	t.last = m
	t.session.AddTurn(m)
	t.refreshStatusLocked()
}

func (t *AgentTUI) renderAll() {
	t.mu.Lock()
	t.applyChromeLocked()
	t.mu.Unlock()
	ui.Clear()
	ui.Render(t.stats, t.result, t.sessions, t.input, t.status)
}

// Run enters the gotui event loop until quit. handler receives submit/session events.
func (t *AgentTUI) Run(handler func(PaneEvent) error) error {
	if err := ui.Init(); err != nil {
		return err
	}
	defer ui.Close()

	w, h := ui.TerminalDimensions()
	t.mu.Lock()
	t.width, t.height = w, h
	t.applyChromeLocked()
	t.mu.Unlock()
	t.renderAll()

	uiEvents := ui.PollEvents()
	for {
		if t.quit.Load() {
			return nil
		}
		select {
		case <-t.redrawCh:
			t.renderAll()
		case ev := <-t.eventCh:
			go func(ev PaneEvent) {
				var err error
				if handler != nil {
					err = handler(ev)
				}
				if err != nil {
					t.mu.Lock()
					t.stats.Text = fmt.Sprintf("error: %v", err)
					t.mu.Unlock()
				}
				t.busy.Store(false)
				t.requestRedraw()
			}(ev)
		case e := <-uiEvents:
			if t.handleUIEvent(e, handler) {
				return nil
			}
		}
	}
}

func (t *AgentTUI) handleUIEvent(e ui.Event, handler func(PaneEvent) error) bool {
	switch e.ID {
	case "<C-c>", "<Escape>":
		return true
	case "<C-n>":
		if !t.busy.Load() {
			t.emit(PaneEvent{Action: KeyActionNewSession, Payload: NewSessionKey()})
			t.renderAll()
		}
		return false
	case "<MouseLeft>":
		t.handleMouseClick(e)
		t.renderAll()
		return false
	case "<MouseWheelUp>", "<MouseWheelDown>":
		t.handleMouseWheel(e)
		t.renderAll()
		return false
	case "<Resize>":
		if payload, ok := e.Payload.(ui.Resize); ok {
			t.mu.Lock()
			t.width, t.height = payload.Width, payload.Height
			if t.pretty && t.plainBuf != "" {
				t.setResultPrettyLocked(t.plainBuf)
			} else {
				t.setResultPlainLocked(t.plainBuf)
			}
			t.rebuildTreeLocked()
			t.mu.Unlock()
			t.renderAll()
		}
		return false
	case "<Tab>":
		t.mu.Lock()
		t.focus = t.focus.Next()
		t.highlightFocusLocked()
		t.mu.Unlock()
		t.renderAll()
		return false
	case "<Backtab>", "<S-Tab>":
		t.mu.Lock()
		t.focus = t.focus.Prev()
		t.highlightFocusLocked()
		t.mu.Unlock()
		t.renderAll()
		return false
	}

	t.mu.Lock()
	focus := t.focus
	busy := t.busy.Load()
	t.mu.Unlock()

	switch focus {
	case FocusResult:
		t.handleResultKey(e)
		t.renderAll()
	case FocusSessions:
		if t.handleSessionsKey(e) {
			t.renderAll()
		}
	default:
		if t.handleInputKey(e, busy, handler) {
			return true
		}
		t.renderAll()
	}
	return false
}

func (t *AgentTUI) handleMouseClick(e ui.Event) {
	m, ok := e.Payload.(ui.Mouse)
	if !ok {
		return
	}
	t.mu.Lock()
	c := ComputeChrome(t.width, t.height, t.inputRows)
	focus := HitTestPane(c, m.X, m.Y)
	if focus == FocusNone {
		t.mu.Unlock()
		return
	}
	t.focus = focus
	t.highlightFocusLocked()

	if focus == FocusSessions && PointInRect(c.Sessions, m.X, m.Y) {
		innerY := t.sessions.Inner.Min.Y
		rel := m.Y - innerY
		if rel >= 0 {
			idx := t.treeTop + rel
			if idx >= 0 && idx < len(t.treeRows) {
				t.treeSel = idx
				t.refreshSessionsViewLocked()
				row := t.treeRows[idx]
				t.mu.Unlock()
				t.activateTreeRow(row)
				return
			}
		}
	}
	t.mu.Unlock()
}

func (t *AgentTUI) handleMouseWheel(e ui.Event) {
	m, ok := e.Payload.(ui.Mouse)
	if !ok {
		return
	}
	t.mu.Lock()
	c := ComputeChrome(t.width, t.height, t.inputRows)
	dir := 1
	if e.ID == "<MouseWheelUp>" {
		dir = -1
	}
	switch {
	case PointInRect(c.Result, m.X, m.Y) || t.focus == FocusResult:
		h := maxInt(1, t.result.Inner.Dy())
		t.resultOff = ClampOffset(t.resultOff+dir, len(t.resultLines), h)
		t.refreshResultViewLocked()
		if PointInRect(c.Result, m.X, m.Y) {
			t.focus = FocusResult
			t.highlightFocusLocked()
		}
	case PointInRect(c.Sessions, m.X, m.Y) || t.focus == FocusSessions:
		h := maxInt(1, t.sessions.Inner.Dy())
		t.treeTop = ClampOffset(t.treeTop+dir, len(t.treeRows), h)
		t.refreshSessionsViewLocked()
		if PointInRect(c.Sessions, m.X, m.Y) {
			t.focus = FocusSessions
			t.highlightFocusLocked()
		}
	}
	t.mu.Unlock()
}

func (t *AgentTUI) activateTreeRow(row SessionTreeRow) {
	switch row.Kind {
	case TreeRowSession:
		// Toggle expand when already current; otherwise switch session.
		t.mu.Lock()
		if row.SessionKey == t.currentKey {
			t.treeExpanded[row.SessionKey] = !t.treeExpanded[row.SessionKey]
			t.rebuildTreeLocked()
			t.mu.Unlock()
			t.requestRedraw()
			return
		}
		t.mu.Unlock()
		t.emit(PaneEvent{Action: KeyActionSwitchSession, Payload: row.SessionKey})
	case TreeRowRequest, TreeRowResponse:
		t.ShowSessionContent(row.Content)
		t.mu.Lock()
		t.focus = FocusResult
		t.highlightFocusLocked()
		t.mu.Unlock()
		t.requestRedraw()
	}
}

func (t *AgentTUI) handleResultKey(e ui.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	h := maxInt(1, t.result.Inner.Dy())
	switch e.ID {
	case "j", "<Down>", "<MouseWheelDown>":
		t.resultOff = ClampOffset(t.resultOff+1, len(t.resultLines), h)
	case "k", "<Up>", "<MouseWheelUp>":
		t.resultOff = ClampOffset(t.resultOff-1, len(t.resultLines), h)
	case "<PageDown>":
		t.resultOff = PageDown(t.resultOff, len(t.resultLines), h)
	case "<PageUp>":
		t.resultOff = PageUp(t.resultOff, h)
	case "g", "<Home>":
		t.resultOff = 0
	case "G", "<End>":
		t.resultOff = ClampOffset(len(t.resultLines), len(t.resultLines), h)
	}
	t.refreshResultViewLocked()
}

func (t *AgentTUI) handleSessionsKey(e ui.Event) bool {
	t.mu.Lock()
	n := len(t.treeRows)
	if n == 0 {
		t.mu.Unlock()
		return false
	}
	h := maxInt(1, t.sessions.Inner.Dy())
	switch e.ID {
	case "j", "<Down>":
		if t.treeSel < n-1 {
			t.treeSel++
		}
		t.refreshSessionsViewLocked()
		t.mu.Unlock()
		return true
	case "k", "<Up>":
		if t.treeSel > 0 {
			t.treeSel--
		}
		t.refreshSessionsViewLocked()
		t.mu.Unlock()
		return true
	case "<PageDown>":
		t.treeSel = ClampOffset(t.treeSel+h, n, 1)
		t.refreshSessionsViewLocked()
		t.mu.Unlock()
		return true
	case "<PageUp>":
		t.treeSel = ClampOffset(t.treeSel-h, n, 1)
		t.refreshSessionsViewLocked()
		t.mu.Unlock()
		return true
	case "<Enter>", "<Space>", "l", "<Right>":
		idx := t.treeSel
		var row SessionTreeRow
		if idx >= 0 && idx < n {
			row = t.treeRows[idx]
		}
		t.mu.Unlock()
		if row.SessionKey != "" || row.Content != "" {
			t.activateTreeRow(row)
		}
		return true
	case "h", "<Left>":
		idx := t.treeSel
		if idx >= 0 && idx < n && t.treeRows[idx].Kind == TreeRowSession {
			key := t.treeRows[idx].SessionKey
			t.treeExpanded[key] = false
			t.rebuildTreeLocked()
		}
		t.mu.Unlock()
		return true
	case "n":
		t.mu.Unlock()
		t.emit(PaneEvent{Action: KeyActionNewSession, Payload: NewSessionKey()})
		return true
	default:
		t.mu.Unlock()
		return false
	}
}

func (t *AgentTUI) handleInputKey(e ui.Event, busy bool, _ func(PaneEvent) error) bool {
	switch e.ID {
	case "<Enter>":
		if busy || !AllowKey(FocusInput, busy, "enter") {
			return false
		}
		text := strings.TrimSpace(t.input.Text)
		if text == "" {
			return false
		}
		if text == "exit" || text == "quit" {
			t.quit.Store(true)
			return true
		}
		t.input.Text = ""
		t.input.Cursor.X = 0
		t.input.Cursor.Y = 0
		t.busy.Store(true)
		t.emit(PaneEvent{Action: KeyActionSubmit, Payload: text})
		return false
	case "<C-j>":
		t.input.InsertNewline()
		return false
	case "<Backspace>", "<Delete>":
		t.input.DeleteRune()
		return false
	case "<Left>":
		t.input.MoveCursor(-1, 0)
		return false
	case "<Right>":
		t.input.MoveCursor(1, 0)
		return false
	case "<Up>":
		t.input.MoveCursor(0, -1)
		return false
	case "<Down>":
		t.input.MoveCursor(0, 1)
		return false
	case "<Space>":
		t.input.InsertRune(' ')
		return false
	default:
		if r, ok := eventRune(e.ID); ok {
			t.input.InsertRune(r)
		}
		return false
	}
}

func (t *AgentTUI) emit(ev PaneEvent) {
	select {
	case t.eventCh <- ev:
	default:
		go func() { t.eventCh <- ev }()
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
