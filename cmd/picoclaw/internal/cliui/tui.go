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

	mu       sync.Mutex
	focus    Focus
	width    int
	height   int
	inputRows int

	resultLines []string
	resultOff   int
	plainBuf    string
	pretty      bool

	sessionItems []SessionItem
	sessionKeys  []string

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
	status.Border = false
	status.Text = "↑in ↓out · elapsed · tps · waiting for first turn"

	if prompt == "" {
		prompt = "You: "
	}

	return &AgentTUI{
		prompt:    prompt,
		stats:     stats,
		result:    result,
		sessions:  sessions,
		input:     input,
		status:    status,
		focus:     FocusInput,
		width:     80,
		height:    24,
		inputRows: 3,
		redrawCh:  make(chan struct{}, 1),
		eventCh:   make(chan PaneEvent, 8),
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
	t.highlightFocusLocked()
}

func setWidgetRect(w interface{ SetRect(int, int, int, int) }, r Rect) {
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
	w := t.result.Inner.Dx()
	if w < 20 {
		w = 40
	}
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

// SyncSessions refreshes the right-hand list from a lister.
func (t *AgentTUI) SyncSessions(src SessionLister, currentKey string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tw := 16
	if t.sessions.Inner.Dx() > 4 {
		tw = t.sessions.Inner.Dx() - 4
	}
	items := BuildSessionItems(src, currentKey, tw)
	t.sessionItems = items
	t.sessionKeys = make([]string, len(items))
	rows := make([]string, len(items))
	sel := 0
	for i, it := range items {
		t.sessionKeys[i] = it.Key
		mark := " "
		if it.Key == currentKey {
			mark = "●"
			sel = i
		}
		rows[i] = fmt.Sprintf("%s %s", mark, it.Title)
	}
	t.sessions.Rows = rows
	t.sessions.SelectedRow = sel
	t.requestRedraw()
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
	case "<Resize>":
		if payload, ok := e.Payload.(ui.Resize); ok {
			t.mu.Lock()
			t.width, t.height = payload.Width, payload.Height
			if t.pretty && t.plainBuf != "" {
				t.setResultPrettyLocked(t.plainBuf)
			} else {
				t.setResultPlainLocked(t.plainBuf)
			}
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
	n := len(t.sessions.Rows)
	if n == 0 {
		t.mu.Unlock()
		return false
	}
	switch e.ID {
	case "j", "<Down>":
		if t.sessions.SelectedRow < n-1 {
			t.sessions.SelectedRow++
		}
		t.mu.Unlock()
		return true
	case "k", "<Up>":
		if t.sessions.SelectedRow > 0 {
			t.sessions.SelectedRow--
		}
		t.mu.Unlock()
		return true
	case "<Enter>":
		idx := t.sessions.SelectedRow
		var key string
		if idx >= 0 && idx < len(t.sessionKeys) {
			key = t.sessionKeys[idx]
		}
		t.mu.Unlock()
		if key != "" {
			t.emit(PaneEvent{Action: KeyActionSwitchSession, Payload: key})
		}
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
		if len(e.ID) == 1 {
			t.input.InsertRune([]rune(e.ID)[0])
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
