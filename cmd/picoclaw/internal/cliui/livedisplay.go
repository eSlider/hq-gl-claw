package cliui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/sipeed/picoclaw/pkg/bus"
)

// EstimateTokens approximates token count as ceil(runes/4). Empty → 0.
func EstimateTokens(s string) int {
	n := utf8.RuneCountInString(s)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4
}

// TPS returns tokens per second for the given duration.
func TPS(tokens int, d time.Duration) float64 {
	if tokens <= 0 || d <= 0 {
		return 0
	}
	return float64(tokens) / d.Seconds()
}

// FormatTPSLine formats a compact status footer: "42 tok · 28.5 tps".
func FormatTPSLine(tokens int, tps float64) string {
	return fmt.Sprintf("%d tok · %.1f tps", tokens, tps)
}

// ProgressBar is a lightweight indeterminate CLI progress indicator.
type ProgressBar struct {
	width int
	label string
}

func NewProgressBar(width int) *ProgressBar {
	if width < 8 {
		width = 8
	}
	return &ProgressBar{width: width, label: "Thinking"}
}

// Render returns an animated frame for tick n.
func (p *ProgressBar) Render(tick int) string {
	if p == nil {
		return ""
	}
	inner := p.width - 2
	if inner < 4 {
		inner = 4
	}
	pos := tick % (inner * 2)
	if pos >= inner {
		pos = inner*2 - pos - 1
	}
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < inner; i++ {
		if i == pos {
			b.WriteByte('>')
		} else if i < pos {
			b.WriteByte('=')
		} else {
			b.WriteByte(' ')
		}
	}
	b.WriteByte(']')
	b.WriteByte(' ')
	b.WriteString(p.label)
	b.WriteString("…")
	return b.String()
}

// LiveDisplay streams agent text to out while showing progress on errW.
type LiveDisplay struct {
	out  io.Writer
	errW io.Writer
	logo string
	bar  *ProgressBar

	mu       sync.Mutex
	ioMu     sync.Mutex
	started  bool
	finished bool
	last     string
	firstAt  time.Time
	startAt  time.Time
	stopProg chan struct{}
	progDone chan struct{}

	typeDelay time.Duration
	streamed  atomic.Bool

	promptEst int
	inTokens  int
	outTokens int
	inExact   bool
	outExact  bool
}

func NewLiveDisplay(out, errW io.Writer, logo string) *LiveDisplay {
	if out == nil {
		out = io.Discard
	}
	if errW == nil {
		errW = io.Discard
	}
	return &LiveDisplay{
		out:       out,
		errW:      errW,
		logo:      logo,
		bar:       NewProgressBar(16),
		typeDelay: 8 * time.Millisecond,
	}
}

func (d *LiveDisplay) writeOut(s string) {
	d.ioMu.Lock()
	defer d.ioMu.Unlock()
	_, _ = io.WriteString(d.out, s)
}

func (d *LiveDisplay) writeErr(s string) {
	d.ioMu.Lock()
	defer d.ioMu.Unlock()
	_, _ = io.WriteString(d.errW, s)
}

// Start shows the logo and begins the progress animation on stderr.
func (d *LiveDisplay) Start() {
	d.StartPrompt("")
}

// StartPrompt is Start with an optional user prompt for ↑ token estimate.
func (d *LiveDisplay) StartPrompt(prompt string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.started {
		return
	}
	d.started = true
	d.startAt = time.Now()
	if prompt != "" {
		d.promptEst = EstimateTokens(prompt)
		d.inTokens = d.promptEst
		d.inExact = false
	}
	d.stopProg = make(chan struct{})
	d.progDone = make(chan struct{})
	d.writeOut(fmt.Sprintf("\n%s\n", d.logo))
	go d.animateProgress(d.stopProg, d.progDone)
}

// SetTurnUsage records provider-reported prompt/completion tokens.
func (d *LiveDisplay) SetTurnUsage(in, out int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if in > 0 {
		d.inTokens = in
		d.inExact = true
	}
	if out > 0 {
		d.outTokens = out
		d.outExact = true
	}
}

func (d *LiveDisplay) animateProgress(stop <-chan struct{}, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	tick := 0
	for {
		select {
		case <-stop:
			d.writeErr("\r" + strings.Repeat(" ", 48) + "\r")
			return
		case <-ticker.C:
			if d.streamed.Load() {
				continue
			}
			d.writeErr("\r" + d.bar.Render(tick))
			tick++
		}
	}
}

func (d *LiveDisplay) stopProgress() {
	d.mu.Lock()
	ch := d.stopProg
	done := d.progDone
	d.stopProg = nil
	d.progDone = nil
	d.mu.Unlock()
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

func (d *LiveDisplay) signalFirstToken() {
	if d.streamed.Swap(true) {
		return
	}
	d.mu.Lock()
	d.firstAt = time.Now()
	ch := d.stopProg
	d.mu.Unlock()
	if ch != nil {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
	d.writeErr("\r" + strings.Repeat(" ", 48) + "\r")
}

// Update implements bus.Streamer — content is the accumulated answer so far.
func (d *LiveDisplay) Update(_ context.Context, content string) error {
	d.mu.Lock()
	if d.finished {
		d.mu.Unlock()
		return nil
	}
	if !d.started {
		d.started = true
		d.startAt = time.Now()
		d.stopProg = make(chan struct{})
		d.progDone = make(chan struct{})
		d.writeOut(fmt.Sprintf("\n%s\n", d.logo))
		go d.animateProgress(d.stopProg, d.progDone)
	}
	last := d.last
	d.mu.Unlock()

	d.signalFirstToken()

	d.mu.Lock()
	defer d.mu.Unlock()
	if strings.HasPrefix(content, last) {
		delta := content[len(last):]
		if delta != "" {
			d.writeOut(delta)
		}
	} else if content != "" {
		d.writeOut("\n" + content)
	}
	d.last = content
	if !d.outExact {
		d.outTokens = EstimateTokens(content)
	}
	return nil
}

// Finalize implements bus.Streamer.
func (d *LiveDisplay) Finalize(_ context.Context, content string) error {
	d.Finish(content)
	return nil
}

// Cancel implements bus.Streamer.
func (d *LiveDisplay) Cancel(context.Context) {
	d.stopProgress()
}

// Finish ends the turn: typewriter fallback if nothing was streamed, then tps.
func (d *LiveDisplay) Finish(content string) {
	d.stopProgress()

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.finished {
		return
	}
	d.finished = true

	if !d.started {
		d.started = true
		d.startAt = time.Now()
		d.writeOut(fmt.Sprintf("\n%s\n", d.logo))
	}

	if !d.streamed.Load() {
		d.firstAt = time.Now()
		d.streamed.Store(true)
		delay := d.typeDelay
		d.mu.Unlock()
		for _, r := range content {
			d.writeOut(string(r))
			if delay > 0 {
				time.Sleep(delay)
			}
		}
		d.mu.Lock()
		d.last = content
	} else if content != "" && content != d.last {
		if strings.HasPrefix(content, d.last) {
			d.writeOut(content[len(d.last):])
		}
		d.last = content
	}

	elapsed := time.Since(d.startAt)
	if !d.outExact {
		d.outTokens = EstimateTokens(d.last)
	}
	if !d.inExact && d.inTokens == 0 {
		d.inTokens = d.promptEst
	}
	m := TurnMetrics{
		PromptTokens:     d.inTokens,
		CompletionTokens: d.outTokens,
		PromptExact:      d.inExact,
		CompletionExact:  d.outExact,
		Elapsed:          elapsed,
	}
	if !d.firstAt.IsZero() {
		m.TTFT = d.firstAt.Sub(d.startAt)
	}
	d.writeOut("\n" + FormatTurnStatus(m) + "\n")
}

// Streamed reports whether any live delta was received from the provider.
func (d *LiveDisplay) Streamed() bool {
	return d.streamed.Load()
}

type streamDelegate struct {
	display *LiveDisplay
}

// NewStreamDelegate returns a StreamDelegate that serves display for CLI turns.
func NewStreamDelegate(display *LiveDisplay) bus.StreamDelegate {
	return &streamDelegate{display: display}
}

func (d *streamDelegate) GetStreamer(_ context.Context, channel, _, _ string) (bus.Streamer, bool) {
	if d == nil || d.display == nil || channel != "cli" {
		return nil, false
	}
	return d.display, true
}
