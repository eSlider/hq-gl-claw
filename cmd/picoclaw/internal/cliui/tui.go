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

	progress *widgets.Paragraph // bottom-right: emoji / streaming
	result   *widgets.Paragraph
	sessions *widgets.List
	input    *widgets.TextArea
	status   *widgets.Paragraph // bottom-left: turn metrics
	help     *widgets.Paragraph // Ctrl+H / F1 shortcut overlay

	mu        sync.Mutex
	focus     Focus
	width     int
	height    int
	inputRows int
	showHelp  bool

	resultLines []string
	resultOff   int
	plainBuf    string
	pretty      bool

	treeRows      []SessionTreeRow
	treeSel       int
	treeTop       int
	treeHover     int             // row under mouse; -1 if none
	treeExpanded  map[string]bool // node id or session key → expanded
	retainKeys    map[string]struct{}
	forest        map[string]*ConvNode
	idGen         idGen
	selectedID    string // currently selected tree node
	pendingReqID  string // in-flight request node awaiting response
	sessionLister SessionLister
	currentKey    string

	transcriptPlain  string
	transcriptBlocks []TranscriptBlock
	highlightKind    TreeRowKind
	highlightContent string
	hasHighlight     bool

	sel textSel // drag-select in result pane → clipboard

	session SessionMetrics
	last    TurnMetrics
	busy    atomic.Bool
	quit    atomic.Bool

	redrawCh chan struct{}
	eventCh  chan PaneEvent
}

// newParagraph builds a bordered paragraph panel with a title.
func newParagraph(title string) *widgets.Paragraph {
	p := widgets.NewParagraph()
	p.Title = title
	p.Border = true
	p.BorderRounded = true
	return p
}

// NewAgentTUI builds the interactive TUI (not yet initialized on the terminal).
func NewAgentTUI(prompt string) *AgentTUI {
	progress := newParagraph("")
	progress.Text = "ready"
	progress.TextStyle = ui.NewStyle(colorProgress)

	result := newParagraph("result")
	result.WrapText = false

	sessions := widgets.NewList()
	sessions.Title = "sessions"
	sessions.Border = true
	sessions.BorderRounded = true
	sessions.SelectedStyle = ui.NewStyle(ui.ColorBlack, colorAccent)

	input := widgets.NewTextArea()
	input.Title = inputHint
	input.Border = true
	input.BorderRounded = true
	input.ShowCursor = true
	input.Text = ""

	status := newParagraph("")
	status.Text = "↑in ↓out · elapsed · tps · waiting for first turn"

	help := newParagraph("help · Ctrl+H / Esc close")
	help.Text = HelpShortcuts
	help.WrapText = false
	help.BorderStyle.Fg = colorAccent
	help.TextStyle = ui.NewStyle(colorIdle)

	if prompt == "" {
		prompt = "You: "
	}

	return &AgentTUI{
		prompt:       prompt,
		progress:     progress,
		result:       result,
		sessions:     sessions,
		input:        input,
		status:       status,
		help:         help,
		focus:        FocusInput,
		width:        80,
		height:       24,
		inputRows:    3,
		treeHover:    -1,
		treeExpanded: map[string]bool{},
		retainKeys:   map[string]struct{}{},
		forest:       map[string]*ConvNode{},
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
	setWidgetRect(t.result, c.Result)
	setWidgetRect(t.sessions, c.Sessions)
	setWidgetRect(t.input, c.Input)
	setWidgetRect(t.status, c.Status)
	setWidgetRect(t.progress, c.Progress)
	if t.help != nil {
		hw, hh := HelpOverlaySize(t.width, t.height)
		setWidgetRect(t.help, CenterRect(t.width, t.height, hw, hh))
	}
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
	idle := ui.NewStyle(colorIdle)
	active := ui.NewStyle(colorAccent)
	t.result.BorderStyle.Fg = idle.Fg
	t.sessions.BorderStyle.Fg = idle.Fg
	t.input.BorderStyle.Fg = idle.Fg
	t.status.BorderStyle.Fg = idle.Fg
	t.progress.BorderStyle.Fg = idle.Fg
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
	lines := t.resultLines
	if t.sel.active && !t.sel.empty() {
		l0, c0, l1, c1 := t.sel.normalized()
		lines = ApplySelectionHighlight(t.resultLines, l0, c0, l1, c1)
	}
	t.resultOff = ClampOffset(t.resultOff, len(t.resultLines), h)
	t.result.Text = ViewWindow(lines, t.resultOff, h)
}

func (t *AgentTUI) setResultPlainLocked(content string) {
	t.plainBuf = content
	t.pretty = false
	t.transcriptPlain = ""
	t.transcriptBlocks = nil
	t.hasHighlight = false
	t.resultLines = strings.Split(content, "\n")
	t.resultOff = maxInt(0, len(t.resultLines)-maxInt(1, t.result.Inner.Dy()))
	t.refreshResultViewLocked()
}

func (t *AgentTUI) setResultPrettyLocked(content string) {
	t.plainBuf = content
	t.pretty = true
	t.transcriptPlain = ""
	t.transcriptBlocks = nil
	t.hasHighlight = false
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

func (t *AgentTUI) setTranscriptLocked(msgs []ChatMessage) {
	text, blocks := FormatSessionTranscript(msgs)
	t.transcriptPlain = text
	t.transcriptBlocks = blocks
	t.plainBuf = text
	t.pretty = false
	t.hasHighlight = false
	t.resultLines = strings.Split(text, "\n")
	if text == "" {
		t.resultLines = nil
	}
	t.resultOff = maxInt(0, len(t.resultLines)-maxInt(1, t.result.Inner.Dy()))
	t.refreshResultViewLocked()
}

func (t *AgentTUI) applyHoverHighlightLocked(kind TreeRowKind, content string) {
	if t.transcriptPlain == "" || len(t.transcriptBlocks) == 0 {
		return
	}
	bl, ok := FindTranscriptBlock(t.transcriptBlocks, kind, content)
	if !ok {
		return
	}
	base := strings.Split(t.transcriptPlain, "\n")
	if t.transcriptPlain == "" {
		base = nil
	}
	w := t.result.Inner.Dx()
	if w < 20 {
		w = 40
	}
	t.resultLines = ApplyThinHighlight(base, bl.LineStart, bl.LineEnd, w)
	t.highlightKind = kind
	t.highlightContent = content
	t.hasHighlight = true
	h := maxInt(1, t.result.Inner.Dy())
	// Account for the inserted top border line shifting the block down by 1.
	t.resultOff = ScrollToHighlightOffset(bl.LineStart, len(t.resultLines), h)
	t.refreshResultViewLocked()
}

func (t *AgentTUI) clearHoverHighlightLocked() {
	if !t.hasHighlight || t.transcriptPlain == "" {
		return
	}
	t.hasHighlight = false
	t.resultLines = strings.Split(t.transcriptPlain, "\n")
	t.refreshResultViewLocked()
}

// SyncSessions refreshes the right-hand session tree from a lister.
func (t *AgentTUI) SyncSessions(src SessionLister, currentKey string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	prev := t.currentKey
	t.sessionLister = src
	if currentKey != "" {
		t.retainKeys[currentKey] = struct{}{}
	}
	if src != nil {
		for _, k := range src.ListSessions() {
			// CLI pane only tracks cli:* sessions — loading every channel
			// session on startup made SyncSessions feel like a hang.
			if k != "" && strings.HasPrefix(k, "cli:") {
				t.retainKeys[k] = struct{}{}
			}
		}
	}
	alreadyFocused := prev == currentKey && currentKey != ""
	t.focusSessionLocked(prev, currentKey)
	t.rebuildTreeLocked()
	// Avoid clobbering a transcript/highlight the activator just loaded when the
	// event handler re-syncs after an in-UI session switch.
	if !alreadyFocused {
		t.loadTranscriptForCurrentLocked()
	}
	t.requestRedraw()
}

// RetainSession keeps key visible in the sessions tree across Ctrl+N / rebuilds.
func (t *AgentTUI) RetainSession(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.retainKeys == nil {
		t.retainKeys = map[string]struct{}{}
	}
	t.retainKeys[key] = struct{}{}
	t.rebuildTreeLocked()
	t.requestRedraw()
}

// focusSessionLocked switches currentKey, collapses other sessions, and clears
// stale selection/input when the active session changes.
func (t *AgentTUI) focusSessionLocked(prev, key string) {
	t.currentKey = key
	for k := range t.retainKeys {
		if k == key {
			continue
		}
		t.treeExpanded[k] = false
		if root := t.forest[k]; root != nil {
			t.treeExpanded[root.ID] = false
		}
	}
	if key != "" {
		t.treeExpanded[key] = true
	}
	if n := t.nodeByIDLocked(t.selectedID); n == nil || n.Session != key {
		t.selectedID = ""
	}
	if prev != key {
		t.input.Text = ""
		t.input.Cursor.X = 0
		t.input.Cursor.Y = 0
		t.hasHighlight = false
		t.treeHover = -1
		t.pendingReqID = ""
	}
}

func (t *AgentTUI) nodeByIDLocked(id string) *ConvNode {
	if id == "" {
		return nil
	}
	for _, root := range t.forest {
		if n := FindNode(root, id); n != nil {
			return n
		}
	}
	return nil
}

func (t *AgentTUI) loadTranscriptForCurrentLocked() {
	root := t.forest[t.currentKey]
	if root == nil {
		t.setTranscriptLocked(nil)
		return
	}
	tip := DeepestTip(root)
	if tip == nil || tip.Kind == TreeRowSession {
		t.setTranscriptLocked(nil)
		return
	}
	t.setTranscriptLocked(PathMessages(tip))
}

func (t *AgentTUI) retainKeyListLocked() []string {
	out := make([]string, 0, len(t.retainKeys))
	for k := range t.retainKeys {
		out = append(out, k)
	}
	return out
}

func (t *AgentTUI) rebuildTreeLocked() {
	tw := 16
	if t.sessions.Inner.Dx() > 4 {
		tw = t.sessions.Inner.Dx() - 4
	}
	var rows []SessionTreeRow
	rows, t.forest = BuildSessionTreeRows(
		t.sessionLister, t.currentKey, tw, t.forest, t.treeExpanded, &t.idGen, t.retainKeyListLocked()...,
	)
	t.treeRows = rows
	// Prefer keeping selected node only if it belongs to the current session.
	sel := 0
	found := false
	for i, r := range t.treeRows {
		if t.selectedID != "" && r.NodeID == t.selectedID && r.SessionKey == t.currentKey {
			sel = i
			found = true
			break
		}
	}
	if !found {
		t.selectedID = ""
		if tip := DeepestTip(t.forest[t.currentKey]); tip != nil {
			t.selectedID = tip.ID
			ExpandAncestors(t.treeExpanded, tip)
			rows, t.forest = BuildSessionTreeRows(
				t.sessionLister, t.currentKey, tw, t.forest, t.treeExpanded, &t.idGen, t.retainKeyListLocked()...,
			)
			t.treeRows = rows
			for i, r := range t.treeRows {
				if r.NodeID == t.selectedID {
					sel = i
					found = true
					break
				}
			}
		}
	}
	if !found {
		for i, r := range t.treeRows {
			if r.Kind == TreeRowSession && r.SessionKey == t.currentKey {
				sel = i
				t.selectedID = r.NodeID
				break
			}
		}
	}
	t.treeSel = sel
	t.refreshSessionsViewLocked()
}

// CompleteRequest attaches the assistant response under the pending request node.
func (t *AgentTUI) CompleteRequest(requestNodeID, response string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	id := requestNodeID
	if id == "" {
		id = t.pendingReqID
	}
	t.pendingReqID = ""
	req := t.nodeByIDLocked(id)
	if req == nil {
		return
	}
	if req.Session != "" {
		t.currentKey = req.Session
	}
	resp := AttachResponse(req, response, &t.idGen)
	if resp != nil {
		ExpandAncestors(t.treeExpanded, resp)
		t.selectedID = resp.ID
	}
	if root := t.forest[req.Session]; root != nil {
		root.Content = sessionTitleFromTree(root)
	}
	t.rebuildTreeLocked()
	if resp != nil {
		t.setTranscriptLocked(PathMessages(resp))
	}
	t.requestRedraw()
}

func (t *AgentTUI) selectedNodeLocked() *ConvNode {
	if t.selectedID == "" {
		return t.forest[t.currentKey]
	}
	if n := t.nodeByIDLocked(t.selectedID); n != nil {
		if n.Session == t.currentKey {
			return n
		}
	}
	return t.forest[t.currentKey]
}

func (t *AgentTUI) autofillFromRequestLocked(content string) {
	t.input.Text = content
	t.input.Cursor.X = 0
	t.input.Cursor.Y = 0
	// Place cursor at end.
	for _, r := range content {
		if r == '\n' {
			t.input.Cursor.Y++
			t.input.Cursor.X = 0
		} else {
			t.input.Cursor.X++
		}
	}
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
	t.refreshTitlesLocked()
}

// refreshTitlesLocked keeps pane titles glanceable: the sessions pane shows how
// many sessions exist, and the result pane shows which conversation it displays.
func (t *AgentTUI) refreshTitlesLocked() {
	count := 0
	for _, r := range t.treeRows {
		if r.Kind == TreeRowSession {
			count++
		}
	}
	t.sessions.Title = fmt.Sprintf("sessions · %d", count)

	title := "result"
	if root := t.forest[t.currentKey]; root != nil {
		if bc := sessionTitleFromTree(root); bc != "" && bc != "(empty)" {
			w := maxInt(8, t.result.Inner.Dx()-12)
			title = "result · " + TruncateTitle(bc, w)
		}
	}
	t.result.Title = title
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

// ShowSessionHistory loads a full ↑/↓ transcript for hover navigation.
func (t *AgentTUI) ShowSessionHistory(msgs []ChatMessage) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.setTranscriptLocked(msgs)
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
	showHelp := t.showHelp
	t.mu.Unlock()
	ui.Clear()
	if showHelp {
		ui.Render(t.result, t.sessions, t.input, t.status, t.progress, t.help)
	} else {
		ui.Render(t.result, t.sessions, t.input, t.status, t.progress)
	}
}

// toggleHelpLocked flips the shortcuts overlay. Caller must hold t.mu.
func (t *AgentTUI) toggleHelpLocked() {
	t.showHelp = !t.showHelp
}

// dismissHelpLocked closes the help overlay if open. Returns true if it was open.
func (t *AgentTUI) dismissHelpLocked() bool {
	if !t.showHelp {
		return false
	}
	t.showHelp = false
	return true
}

// handleHelpMouse dismisses help on any click while the overlay is open.
// Returns true if the event was consumed by the help layer.
func (t *AgentTUI) handleHelpMouse(e ui.Event) bool {
	if _, ok := e.Payload.(ui.Mouse); !ok {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.showHelp {
		return false
	}
	t.showHelp = false
	return true
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
					t.progress.Text = fmt.Sprintf("⚠️ %v", err)
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
	case "<C-h>", "<F1>":
		t.mu.Lock()
		t.toggleHelpLocked()
		t.mu.Unlock()
		t.renderAll()
		return false
	case "<C-c>":
		return true
	case "<Escape>":
		t.mu.Lock()
		closed := t.dismissHelpLocked()
		t.mu.Unlock()
		if closed {
			t.renderAll()
			return false
		}
		return true
	case "<C-n>":
		t.mu.Lock()
		if t.showHelp {
			t.dismissHelpLocked()
			t.mu.Unlock()
			t.renderAll()
			return false
		}
		t.mu.Unlock()
		if !t.busy.Load() {
			t.emit(PaneEvent{Action: KeyActionNewSession, Payload: NewSessionKey()})
			t.renderAll()
		}
		return false
	case "<MouseLeft>":
		if t.handleHelpMouse(e) {
			t.renderAll()
			return false
		}
		t.handleMouseLeft(e)
		t.renderAll()
		return false
	case "<MouseRelease>":
		if t.finishResultSelect(e) {
			t.renderAll()
			return false
		}
		t.mu.Lock()
		helping := t.showHelp
		t.mu.Unlock()
		if helping {
			return false
		}
		// Motion without buttons arrives as MouseRelease; only repaint when hover changes.
		if t.handleMouseHover(e) {
			t.renderAll()
		}
		return false
	case "<MouseWheelUp>", "<MouseWheelDown>":
		t.mu.Lock()
		helping := t.showHelp
		t.mu.Unlock()
		if helping {
			return false
		}
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
		if t.showHelp {
			t.mu.Unlock()
			return false
		}
		t.focus = t.focus.Next()
		t.highlightFocusLocked()
		t.mu.Unlock()
		t.renderAll()
		return false
	case "<Backtab>", "<S-Tab>":
		t.mu.Lock()
		if t.showHelp {
			t.mu.Unlock()
			return false
		}
		t.focus = t.focus.Prev()
		t.highlightFocusLocked()
		t.mu.Unlock()
		t.renderAll()
		return false
	case "<Enter>", "<Space>":
		t.mu.Lock()
		if t.showHelp {
			t.dismissHelpLocked()
			t.mu.Unlock()
			t.renderAll()
			return false
		}
		t.mu.Unlock()
		// Fall through to focus-specific handlers below.
	}

	// While help is open, swallow other keys (except those handled above).
	t.mu.Lock()
	if t.showHelp {
		t.mu.Unlock()
		return false
	}
	t.mu.Unlock()

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

func (t *AgentTUI) handleMouseLeft(e ui.Event) {
	m, ok := e.Payload.(ui.Mouse)
	if !ok {
		return
	}
	t.mu.Lock()
	c := ComputeChrome(t.width, t.height, t.inputRows)

	// Dragging or starting a selection inside the result pane.
	if PointInRect(c.Result, m.X, m.Y) {
		t.focus = FocusResult
		t.highlightFocusLocked()
		line, col, okPos := t.resultPosLocked(m.X, m.Y)
		if !okPos {
			t.mu.Unlock()
			return
		}
		if t.sel.dragging {
			t.sel.bLine, t.sel.bCol = line, col
			t.sel.active = true
			t.refreshResultViewLocked()
			t.mu.Unlock()
			return
		}
		t.sel = textSel{
			active:   true,
			dragging: true,
			aLine:    line,
			aCol:     col,
			bLine:    line,
			bCol:     col,
		}
		t.refreshResultViewLocked()
		t.mu.Unlock()
		return
	}

	// Click outside result clears any selection.
	t.clearResultSelectLocked()

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

// resultPosLocked maps screen coords to a line/column in resultLines.
func (t *AgentTUI) resultPosLocked(x, y int) (line, col int, ok bool) {
	inner := t.result.Inner
	if y < inner.Min.Y || y >= inner.Max.Y || x < inner.Min.X || x >= inner.Max.X {
		return 0, 0, false
	}
	relY := y - inner.Min.Y
	line = t.resultOff + relY
	if line < 0 || line >= len(t.resultLines) {
		// Allow selecting past last line as end-of-buffer.
		if line >= len(t.resultLines) && len(t.resultLines) > 0 {
			line = len(t.resultLines) - 1
			plain := stripGotuiMarkup(t.resultLines[line])
			return line, len([]rune(plain)), true
		}
		return 0, 0, false
	}
	relX := x - inner.Min.X
	plain := stripGotuiMarkup(t.resultLines[line])
	col = RuneIndexAtVisualCol(plain, relX)
	return line, col, true
}

func (t *AgentTUI) clearResultSelectLocked() {
	if !t.sel.active && !t.sel.dragging {
		return
	}
	t.sel = textSel{}
	t.refreshResultViewLocked()
}

// finishResultSelect ends a drag-select and copies the span to the clipboard.
// Returns true when the event was consumed as a selection finish.
func (t *AgentTUI) finishResultSelect(e ui.Event) bool {
	m, ok := e.Payload.(ui.Mouse)
	if !ok {
		return false
	}
	t.mu.Lock()
	if !t.sel.dragging {
		t.mu.Unlock()
		return false
	}
	if line, col, okPos := t.resultPosLocked(m.X, m.Y); okPos {
		t.sel.bLine, t.sel.bCol = line, col
	}
	t.sel.dragging = false
	t.sel.active = true
	if t.sel.empty() {
		t.sel = textSel{}
		t.refreshResultViewLocked()
		t.mu.Unlock()
		return true
	}
	l0, c0, l1, c1 := t.sel.normalized()
	text := ExtractSelection(t.resultLines, l0, c0, l1, c1)
	t.refreshResultViewLocked()
	t.mu.Unlock()
	if text != "" {
		CopyToClipboard(text)
		t.mu.Lock()
		t.progress.Text = "📋 copied"
		t.mu.Unlock()
	}
	return true
}

func (t *AgentTUI) handleMouseHover(e ui.Event) bool {
	m, ok := e.Payload.(ui.Mouse)
	if !ok {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.sel.dragging {
		return false
	}
	c := ComputeChrome(t.width, t.height, t.inputRows)
	if !PointInRect(c.Sessions, m.X, m.Y) {
		if t.treeHover != -1 {
			t.treeHover = -1
			t.clearHoverHighlightLocked()
			return true
		}
		return false
	}
	innerY := t.sessions.Inner.Min.Y
	rel := m.Y - innerY
	if rel < 0 {
		return false
	}
	idx := t.treeTop + rel
	if idx < 0 || idx >= len(t.treeRows) {
		return false
	}
	// Same row: hover/highlight already applied — skip redraw (motion floods).
	if idx == t.treeHover {
		return false
	}
	t.treeHover = idx
	row := t.treeRows[idx]
	if row.SessionKey != t.currentKey {
		t.clearHoverHighlightLocked()
		return true
	}
	t.selectedID = row.NodeID
	switch row.Kind {
	case TreeRowRequest, TreeRowResponse:
		t.applyHoverHighlightLocked(row.Kind, row.Content)
	default:
		t.clearHoverHighlightLocked()
	}
	return true
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
		t.mu.Lock()
		same := row.SessionKey == t.currentKey
		if same {
			t.treeExpanded[row.SessionKey] = true
			if row.NodeID != "" {
				t.treeExpanded[row.NodeID] = true
			}
			t.selectedID = row.NodeID
			t.rebuildTreeLocked()
			t.pinTreeSelLocked()
			t.loadTranscriptForCurrentLocked()
			t.mu.Unlock()
			t.requestRedraw()
			return
		}
		prev := t.currentKey
		t.focusSessionLocked(prev, row.SessionKey)
		t.selectedID = row.NodeID
		t.rebuildTreeLocked()
		t.pinTreeSelLocked()
		t.loadTranscriptForCurrentLocked()
		key := row.SessionKey
		t.mu.Unlock()
		t.emit(PaneEvent{Action: KeyActionSwitchSession, Payload: key})
		t.requestRedraw()
	case TreeRowRequest:
		t.mu.Lock()
		switched := t.selectLeafLocked(row, true)
		t.focus = FocusInput
		t.highlightFocusLocked()
		key := row.SessionKey
		t.mu.Unlock()
		if switched {
			t.emit(PaneEvent{Action: KeyActionSwitchSession, Payload: key})
		}
		t.requestRedraw()
	case TreeRowResponse:
		t.mu.Lock()
		switched := t.selectLeafLocked(row, false)
		key := row.SessionKey
		t.mu.Unlock()
		if switched {
			t.emit(PaneEvent{Action: KeyActionSwitchSession, Payload: key})
		}
		t.requestRedraw()
	}
}

// pinTreeSelLocked scrolls the sessions list so the selected row sits near the
// top of the viewport (without reordering the underlying session list).
func (t *AgentTUI) pinTreeSelLocked() {
	h := maxInt(1, t.sessions.Inner.Dy())
	t.treeTop = ClampOffset(t.treeSel, len(t.treeRows), h)
	t.refreshSessionsViewLocked()
}

// selectLeafLocked focuses row's session (if different), selects the node,
// reveals it in the tree, previews its content, and — for requests — optionally
// autofills the input. Returns whether the active session changed.
func (t *AgentTUI) selectLeafLocked(row SessionTreeRow, autofill bool) bool {
	switched := row.SessionKey != t.currentKey
	if switched {
		t.focusSessionLocked(t.currentKey, row.SessionKey)
	}
	t.selectedID = row.NodeID
	ExpandAncestors(t.treeExpanded, t.nodeByIDLocked(row.NodeID))
	t.rebuildTreeLocked()
	if switched {
		t.pinTreeSelLocked()
		t.loadTranscriptForCurrentLocked()
	}
	if autofill {
		t.autofillFromRequestLocked(row.Content)
	}
	t.applyHoverHighlightLocked(row.Kind, row.Content)
	return switched
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
		t.moveTreeSelLocked(1)
		t.mu.Unlock()
		return true
	case "k", "<Up>":
		t.moveTreeSelLocked(-1)
		t.mu.Unlock()
		return true
	case "<PageDown>":
		t.moveTreeSelLocked(h)
		t.mu.Unlock()
		return true
	case "<PageUp>":
		t.moveTreeSelLocked(-h)
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
		if idx >= 0 && idx < n {
			row := t.treeRows[idx]
			if row.NodeID != "" {
				t.treeExpanded[row.NodeID] = false
			}
			if row.Kind == TreeRowSession {
				t.treeExpanded[row.SessionKey] = false
			}
			t.rebuildTreeLocked()
			t.clearHoverHighlightLocked()
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

// moveTreeSelLocked shifts the tree selection by delta rows (clamped) and
// previews the newly selected node in the result pane.
func (t *AgentTUI) moveTreeSelLocked(delta int) {
	n := len(t.treeRows)
	if n == 0 {
		return
	}
	t.treeSel = ClampOffset(t.treeSel+delta, n, 1)
	t.refreshSessionsViewLocked()
	t.previewTreeSelectionLocked()
}

func (t *AgentTUI) previewTreeSelectionLocked() {
	if t.treeSel < 0 || t.treeSel >= len(t.treeRows) {
		return
	}
	row := t.treeRows[t.treeSel]
	if row.SessionKey != t.currentKey {
		t.clearHoverHighlightLocked()
		return
	}
	t.selectedID = row.NodeID
	switch row.Kind {
	case TreeRowRequest:
		t.autofillFromRequestLocked(row.Content)
		t.applyHoverHighlightLocked(row.Kind, row.Content)
	case TreeRowResponse:
		t.applyHoverHighlightLocked(row.Kind, row.Content)
	default:
		t.clearHoverHighlightLocked()
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
		t.mu.Lock()
		ev := t.buildSubmitEventLocked(text)
		t.input.Text = ""
		t.input.Cursor.X = 0
		t.input.Cursor.Y = 0
		t.mu.Unlock()
		t.busy.Store(true)
		t.emit(ev)
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

func (t *AgentTUI) buildSubmitEventLocked(text string) PaneEvent {
	sel := t.selectedNodeLocked()
	// Ensure we have a session root.
	if t.forest[t.currentKey] == nil {
		t.forest[t.currentKey] = BuildConvTreeFromHistory(t.currentKey, "(empty)", nil, &t.idGen)
	}
	if sel == nil || sel.Session != t.currentKey {
		sel = t.forest[t.currentKey]
		t.selectedID = sel.ID
	}
	parent := SubmitParent(sel)
	if parent == nil {
		parent = t.forest[t.currentKey]
	}
	// Expand ancestors so the new leaf is visible.
	for n := parent; n != nil; n = n.Parent {
		t.treeExpanded[n.ID] = true
		if n.Kind == TreeRowSession {
			t.treeExpanded[n.Session] = true
		}
	}
	histBefore := PathMessagesBefore(parent)
	req := AttachRequest(parent, text, &t.idGen)
	t.pendingReqID = req.ID
	t.selectedID = req.ID
	t.treeExpanded[parent.ID] = true
	t.rebuildTreeLocked()
	t.requestRedraw()
	return PaneEvent{
		Action:        KeyActionSubmit,
		Payload:       text,
		SessionKey:    t.currentKey,
		NodeID:        req.ID,
		HistoryBefore: histBefore,
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
