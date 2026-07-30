package cliui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/sipeed/picoclaw/pkg/bus"
)

// PanesEnabled reports whether the pane TUI should be used (TTY + env).
func PanesEnabled() bool {
	if os.Getenv("PICOCLAW_PANES") == "0" {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// PaneUI is an interactive three-pane agent console.
type PaneUI struct {
	in   *os.File
	out  *os.File
	sess *PaneSession

	mu       sync.Mutex
	dirty    atomic.Bool
	quit     atomic.Bool
	busy     atomic.Bool
	oldState *term.State
}

// NewPaneUI builds a pane UI bound to stdin/stdout.
func NewPaneUI(prompt string) (*PaneUI, error) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		w, h = 80, 24
	}
	sess := NewPaneSession(w, h)
	if prompt != "" {
		sess.prompt = prompt
	}
	sess.SetStats("ready")
	return &PaneUI{in: os.Stdin, out: os.Stdout, sess: sess}, nil
}

// Session returns the underlying pane state (for tests / streaming).
func (ui *PaneUI) Session() *PaneSession { return ui.sess }

// EnterRaw switches the terminal into raw mode and alt screen.
func (ui *PaneUI) EnterRaw() error {
	st, err := term.MakeRaw(int(ui.in.Fd()))
	if err != nil {
		return err
	}
	ui.oldState = st
	_, _ = io.WriteString(ui.out, "\x1b[?1049h\x1b[?25l") // alt screen, hide cursor
	ui.redraw()
	return nil
}

// ExitRaw restores the terminal.
func (ui *PaneUI) ExitRaw() {
	_, _ = io.WriteString(ui.out, "\x1b[?25h\x1b[?1049l")
	if ui.oldState != nil {
		_ = term.Restore(int(ui.in.Fd()), ui.oldState)
		ui.oldState = nil
	}
}

func (ui *PaneUI) redraw() {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	frame := ui.sess.Render()
	_, _ = io.WriteString(ui.out, "\x1b[H"+frame)
	ui.dirty.Store(false)
}

func (ui *PaneUI) requestRedraw() {
	ui.dirty.Store(true)
}

// Resize to new terminal size and rerender.
func (ui *PaneUI) Resize(width, height int) {
	ui.mu.Lock()
	ui.sess.Resize(width, height)
	ui.mu.Unlock()
	ui.redraw()
}

// Run loops until quit. onSubmit is called for each entered message; it should
// block until the agent turn completes (streaming via PaneStreamer).
func (ui *PaneUI) Run(onSubmit func(msg string) error) error {
	if err := ui.EnterRaw(); err != nil {
		return err
	}
	defer ui.ExitRaw()

	winCh := make(chan os.Signal, 1)
	signal.Notify(winCh, syscall.SIGWINCH)
	defer signal.Stop(winCh)

	keys := make(chan Key, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- readKeys(ui.in, keys, &ui.quit)
	}()

	redrawTicker := time.NewTicker(33 * time.Millisecond)
	defer redrawTicker.Stop()

	for {
		select {
		case <-winCh:
			if w, h, err := term.GetSize(int(ui.out.Fd())); err == nil {
				ui.Resize(w, h)
			}
		case <-redrawTicker.C:
			if ui.dirty.Load() {
				ui.redraw()
			}
		case err := <-errCh:
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			return nil
		case k, ok := <-keys:
			if !ok {
				return nil
			}
			if k == KeyRune(3) { // Ctrl+C
				ui.quit.Store(true)
				return nil
			}
			if ui.busy.Load() {
				ui.mu.Lock()
				focus := ui.sess.Focus()
				ui.mu.Unlock()
				if focus == FocusInput && k != KeyTab && k != KeyShiftTab {
					continue
				}
			}
			ui.mu.Lock()
			msg, submitted := ui.sess.HandleKey(k)
			ui.mu.Unlock()
			ui.redraw()
			if !submitted {
				continue
			}
			if msg == "exit" || msg == "quit" {
				ui.quit.Store(true)
				return nil
			}
			ui.busy.Store(true)
			ui.mu.Lock()
			ui.sess.SetContent("")
			ui.sess.SetStats("Thinking…")
			ui.mu.Unlock()
			ui.redraw()
			err := onSubmit(msg)
			ui.busy.Store(false)
			if err != nil {
				ui.mu.Lock()
				ui.sess.SetStats("error")
				ui.sess.SetContent(err.Error())
				ui.mu.Unlock()
				ui.redraw()
				continue
			}
			ui.mu.Lock()
			ui.sess.focus = FocusInput
			ui.mu.Unlock()
			ui.redraw()
		}
	}
}

// PaneStreamer streams tokens into the pane content area.
type PaneStreamer struct {
	ui *PaneUI

	mu       sync.Mutex
	last     string
	startAt  time.Time
	firstAt  time.Time
	streamed atomic.Bool
	finished bool

	stopProg chan struct{}
	progDone chan struct{}
	bar      *ProgressBar
}

// NewPaneStreamer returns a Streamer bound to ui.
func NewPaneStreamer(ui *PaneUI) *PaneStreamer {
	return &PaneStreamer{
		ui:      ui,
		startAt: time.Now(),
		bar:     NewProgressBar(16),
	}
}

// Start begins the waiting progress animation on the stats line.
func (s *PaneStreamer) Start() {
	s.mu.Lock()
	s.startAt = time.Now()
	s.stopProg = make(chan struct{})
	s.progDone = make(chan struct{})
	stop, done := s.stopProg, s.progDone
	s.mu.Unlock()
	go s.animateProgress(stop, done)
}

func (s *PaneStreamer) animateProgress(stop <-chan struct{}, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	tick := 0
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if s.streamed.Load() || s.finished {
				continue
			}
			s.ui.mu.Lock()
			s.ui.sess.SetStats(s.bar.Render(tick))
			s.ui.mu.Unlock()
			s.ui.requestRedraw()
			tick++
		}
	}
}

func (s *PaneStreamer) stopProgress() {
	s.mu.Lock()
	ch := s.stopProg
	done := s.progDone
	s.stopProg = nil
	s.progDone = nil
	s.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case <-ch:
	default:
		close(ch)
	}
	if done != nil {
		<-done
	}
}

// NewPaneStreamDelegate serves PaneStreamer for channel "cli".
func NewPaneStreamDelegate(s *PaneStreamer) bus.StreamDelegate {
	return &paneStreamDelegate{s: s}
}

type paneStreamDelegate struct {
	s *PaneStreamer
}

func (d *paneStreamDelegate) GetStreamer(_ context.Context, channel, _, _ string) (bus.Streamer, bool) {
	if d == nil || d.s == nil || channel != "cli" {
		return nil, false
	}
	return d.s, true
}

func (s *PaneStreamer) Update(_ context.Context, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return nil
	}
	if !s.streamed.Swap(true) {
		s.firstAt = time.Now()
		ch := s.stopProg
		if ch != nil {
			select {
			case <-ch:
			default:
				close(ch)
			}
		}
	}
	s.last = content
	s.ui.mu.Lock()
	s.ui.sess.SetContent(content)
	tokens := EstimateTokens(content)
	elapsed := time.Since(s.firstAt)
	if s.firstAt.IsZero() {
		elapsed = time.Since(s.startAt)
	}
	s.ui.sess.SetStats(FormatTPSLine(tokens, TPS(tokens, elapsed)) + "  streaming")
	s.ui.mu.Unlock()
	s.ui.requestRedraw()
	return nil
}

func (s *PaneStreamer) Finalize(_ context.Context, content string) error {
	s.Finish(content)
	return nil
}

func (s *PaneStreamer) Cancel(context.Context) {
	s.stopProgress()
}

// Finish writes final content + tps into the stats line.
func (s *PaneStreamer) Finish(content string) {
	s.stopProgress()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.finished = true
	if content != "" {
		s.last = content
	}
	if !s.streamed.Load() {
		s.firstAt = time.Now()
	}
	elapsed := time.Since(s.firstAt)
	if s.firstAt.IsZero() {
		elapsed = time.Since(s.startAt)
	}
	tokens := EstimateTokens(s.last)
	s.ui.mu.Lock()
	s.ui.sess.SetContent(s.last)
	s.ui.sess.SetStats(FormatTPSLine(tokens, TPS(tokens, elapsed)))
	s.ui.mu.Unlock()
	s.ui.redraw()
}

func readKeys(in io.Reader, out chan<- Key, quit *atomic.Bool) error {
	br := bufio.NewReader(in)
	for !quit.Load() {
		b, err := br.ReadByte()
		if err != nil {
			close(out)
			return err
		}
		switch b {
		case 0x09: // Tab
			out <- KeyTab
		case 0x0d, 0x0a: // Enter
			out <- KeyEnter
		case 0x7f, 0x08:
			out <- KeyBackspace
		case 0x03: // Ctrl+C
			out <- KeyRune(3)
			return nil
		case 0x1b: // Esc / CSI
			k, err := readEscKey(br)
			if err != nil {
				if errors.Is(err, io.EOF) {
					close(out)
					return err
				}
				out <- KeyEsc
				continue
			}
			out <- k
		default:
			if b < 32 {
				continue
			}
			// UTF-8: unread and decode rune
			_ = br.UnreadByte()
			r, _, err := br.ReadRune()
			if err != nil {
				close(out)
				return err
			}
			out <- KeyRune(r)
		}
	}
	close(out)
	return nil
}

func readEscKey(br *bufio.Reader) (Key, error) {
	// Peek next byte with short timeout isn't available; read non-blocking-ish.
	// If no follow-up, treat as Esc. Use buffered peek.
	if br.Buffered() == 0 {
		// tiny wait: try deadline via SetReadDeadline if *os.File — skip; use Esc
		time.Sleep(5 * time.Millisecond)
		if br.Buffered() == 0 {
			return KeyEsc, nil
		}
	}
	b, err := br.ReadByte()
	if err != nil {
		return KeyEsc, err
	}
	if b == '[' {
		return readCSI(br)
	}
	if b == 'O' {
		b2, err := br.ReadByte()
		if err != nil {
			return KeyEsc, err
		}
		switch b2 {
		case 'A':
			return KeyUp, nil
		case 'B':
			return KeyDown, nil
		case 'C':
			return KeyRight, nil
		case 'D':
			return KeyLeft, nil
		case 'H':
			return KeyHome, nil
		case 'F':
			return KeyEnd, nil
		}
	}
	return KeyEsc, nil
}

func readCSI(br *bufio.Reader) (Key, error) {
	var nums []byte
	for {
		b, err := br.ReadByte()
		if err != nil {
			return KeyEsc, err
		}
		switch {
		case b >= '0' && b <= '9':
			nums = append(nums, b)
		case b == ';':
			nums = append(nums, b)
		case b == 'A':
			return KeyUp, nil
		case b == 'B':
			return KeyDown, nil
		case b == 'C':
			return KeyRight, nil
		case b == 'D':
			return KeyLeft, nil
		case b == 'Z':
			return KeyShiftTab, nil
		case b == 'H':
			return KeyHome, nil
		case b == 'F':
			return KeyEnd, nil
		case b == '~':
			code := string(nums)
			switch code {
			case "5":
				return KeyPageUp, nil
			case "6":
				return KeyPageDown, nil
			case "1", "7":
				return KeyHome, nil
			case "4", "8":
				return KeyEnd, nil
			default:
				return KeyEsc, nil
			}
		default:
			return KeyEsc, nil
		}
	}
}

// ParseKeySeq is exported for tests: parse a raw byte sequence into a Key.
func ParseKeySeq(seq string) (Key, error) {
	ch := make(chan Key, 4)
	var quit atomic.Bool
	err := readKeys(strings.NewReader(seq), ch, &quit)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	select {
	case k := <-ch:
		return k, nil
	default:
		return 0, fmt.Errorf("no key")
	}
}
