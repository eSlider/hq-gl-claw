package cliui

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
)

// PaneStreamer streams tokens into the agent TUI result pane.
type PaneStreamer struct {
	ui *AgentTUI

	mu       sync.Mutex
	last     string
	startAt  time.Time
	firstAt  time.Time
	streamed atomic.Bool
	finished bool

	stopProg chan struct{}
	progDone chan struct{}
	bar      *ProgressBar

	promptEst      int
	inTokens       int
	outTokens      int
	inExact        bool
	outExact       bool
	metricsApplied bool
}

// NewPaneStreamer returns a Streamer bound to ui.
func NewPaneStreamer(ui *AgentTUI) *PaneStreamer {
	return &PaneStreamer{
		ui:      ui,
		startAt: time.Now(),
		bar:     NewProgressBar(16),
	}
}

// Start begins the waiting progress animation.
func (s *PaneStreamer) Start(prompt string) {
	s.mu.Lock()
	s.startAt = time.Now()
	s.promptEst = EstimateTokens(prompt)
	s.inTokens = s.promptEst
	s.inExact = false
	s.outTokens = 0
	s.outExact = false
	s.metricsApplied = false
	s.finished = false
	s.streamed.Store(false)
	s.stopProg = make(chan struct{})
	s.progDone = make(chan struct{})
	stop, done := s.stopProg, s.progDone
	s.mu.Unlock()

	s.ui.mu.Lock()
	s.ui.last = TurnMetrics{
		PromptTokens: s.promptEst,
		PromptExact:  false,
		Streaming:    true,
		Elapsed:      0,
	}
	s.ui.progress.Text = EmojiProgress(0)
	s.ui.refreshStatusLocked()
	s.ui.mu.Unlock()
	s.ui.requestRedraw()

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
			s.mu.Lock()
			if s.finished {
				s.mu.Unlock()
				return
			}
			elapsed := time.Since(s.startAt)
			m := s.liveMetricsLocked(elapsed, true)
			streamed := s.streamed.Load()
			outTok := s.outTokens
			s.mu.Unlock()

			s.ui.mu.Lock()
			if streamed {
				emoji := thinkingEmojis[tick%len(thinkingEmojis)]
				tps := TPS(outTok, elapsedSinceFirst(s))
				s.ui.progress.Text = fmt.Sprintf(
					"%s %.1f tps", emoji, tps,
				)
			} else {
				s.ui.progress.Text = EmojiProgress(tick)
			}
			s.ui.last = m
			s.ui.refreshStatusLocked()
			s.ui.mu.Unlock()
			s.ui.requestRedraw()
			tick++
		}
	}
}

func (s *PaneStreamer) liveMetricsLocked(elapsed time.Duration, streaming bool) TurnMetrics {
	m := TurnMetrics{
		PromptTokens:     s.inTokens,
		CompletionTokens: s.outTokens,
		PromptExact:      s.inExact,
		CompletionExact:  s.outExact,
		Elapsed:          elapsed,
		Streaming:        streaming,
	}
	if !s.firstAt.IsZero() {
		m.TTFT = s.firstAt.Sub(s.startAt)
	}
	return m
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

// SetTurnUsage records provider-reported prompt/completion tokens.
func (s *PaneStreamer) SetTurnUsage(in, out int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if in > 0 {
		s.inTokens = in
		s.inExact = true
	}
	if out > 0 {
		s.outTokens = out
		s.outExact = true
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
		// Keep progress animation running for the whole turn (tool gaps, slow tokens).
	}
	s.last = content
	if !s.outExact {
		s.outTokens = EstimateTokens(content)
	}
	elapsed := time.Since(s.startAt)
	m := s.liveMetricsLocked(elapsed, true)

	s.ui.mu.Lock()
	s.ui.setResultPlainLocked(content)
	s.ui.progress.Text = fmt.Sprintf("%s %.1f tps", thinkingEmojis[0], TPS(m.CompletionTokens, elapsedSinceFirst(s)))
	s.ui.last = m
	s.ui.refreshStatusLocked()
	s.ui.mu.Unlock()
	s.ui.requestRedraw()
	return nil
}

func elapsedSinceFirst(s *PaneStreamer) time.Duration {
	if s.firstAt.IsZero() {
		return time.Since(s.startAt)
	}
	return time.Since(s.firstAt)
}

func (s *PaneStreamer) Finalize(_ context.Context, content string) error {
	s.Finish(content)
	return nil
}

func (s *PaneStreamer) Cancel(context.Context) {
	s.stopProgress()
}

// Finish writes final content and applies turn metrics to the status bar.
func (s *PaneStreamer) Finish(content string) {
	s.stopProgress()
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	s.finished = true
	if content != "" {
		s.last = content
	}
	if !s.streamed.Load() {
		s.firstAt = time.Now()
	}
	if !s.outExact {
		s.outTokens = EstimateTokens(s.last)
	}
	if !s.inExact && s.inTokens == 0 {
		s.inTokens = s.promptEst
	}
	elapsed := time.Since(s.startAt)
	m := s.liveMetricsLocked(elapsed, false)
	applied := s.metricsApplied
	s.metricsApplied = true
	last := s.last
	s.mu.Unlock()

	s.ui.mu.Lock()
	s.ui.setResultPrettyLocked(last)
	s.ui.progress.Text = "✅ done"
	s.ui.mu.Unlock()
	if !applied {
		s.ui.applyTurnMetrics(m)
	}
	s.ui.requestRedraw()
}
