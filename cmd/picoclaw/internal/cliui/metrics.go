package cliui

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

// TurnMetrics holds per request/response token and timing stats.
type TurnMetrics struct {
	PromptTokens     int
	CompletionTokens int
	ReasoningTokens  int  // estimated tokens from model thinking/reasoning stream
	PromptExact      bool // true when from provider usage
	CompletionExact  bool
	Elapsed          time.Duration
	Streaming        bool
	TTFT             time.Duration // time to first answer token; 0 if unknown
}

// GenDuration is the answer-generation window used for TPS.
// Prefer Elapsed−TTFT when TTFT is known so thinking time is not charged to TPS.
func (m TurnMetrics) GenDuration() time.Duration {
	if m.TTFT > 0 && m.Elapsed > m.TTFT {
		return m.Elapsed - m.TTFT
	}
	return m.Elapsed
}

// SessionMetrics accumulates turns for the interactive session.
type SessionMetrics struct {
	Turns            int
	PromptTokens     int
	CompletionTokens int
	TotalElapsed     time.Duration
}

// AddTurn folds a completed turn into session totals.
func (s *SessionMetrics) AddTurn(m TurnMetrics) {
	if s == nil {
		return
	}
	s.Turns++
	s.PromptTokens += m.PromptTokens
	s.CompletionTokens += m.CompletionTokens
	s.TotalElapsed += m.Elapsed
}

// FormatTurnStatus renders a compact turn line (go-ollama style).
// Thinking (no answer tokens yet): "⏳ ↑n · 1.2s · thinking" (+ ↓think~ when known).
// Generating / done: TPS uses GenDuration (excludes TTFT/think), not wall clock.
func FormatTurnStatus(m TurnMetrics) string {
	marker := "✓"
	if m.Streaming {
		marker = "⏳"
	}
	up := formatTok("↑", m.PromptTokens, m.PromptExact)
	elapsed := formatElapsed(m.Elapsed)

	// Pre-answer phase: show thinking, never a bogus 0.0 tps.
	if m.Streaming && m.CompletionTokens == 0 {
		if m.ReasoningTokens > 0 {
			return fmt.Sprintf("%s %s ↓think~%d · %s · thinking", marker, up, m.ReasoningTokens, elapsed)
		}
		return fmt.Sprintf("%s %s · %s · thinking", marker, up, elapsed)
	}

	down := formatTok("↓", m.CompletionTokens, m.CompletionExact)
	tps := TPS(m.CompletionTokens, m.GenDuration())
	line := fmt.Sprintf("%s %s %s · %s · %.1f tps", marker, up, down, elapsed, tps)
	if m.TTFT > 0 && !m.Streaming {
		line += fmt.Sprintf(" · ttft %s", formatElapsed(m.TTFT))
	}
	return line
}

// FormatStatusBar builds the bottom overall status line.
// endpoint and activity are optional ("bonsai · host", "◐ tool read_file · 1.2s").
func FormatStatusBar(last TurnMetrics, sess SessionMetrics, focus string, extras ...string) string {
	parts := []string{FormatTurnStatus(last)}
	endpoint, activity := "", ""
	if len(extras) > 0 {
		endpoint = extras[0]
	}
	if len(extras) > 1 {
		activity = extras[1]
	}
	// Show in-flight work early so long tool/LLM gaps are obvious.
	if activity != "" {
		parts = append(parts, activity)
	}
	if sess.Turns > 0 {
		parts = append(parts, fmt.Sprintf(
			"Σ ↑%s ↓%s · %d turns",
			formatCompact(sess.PromptTokens),
			formatCompact(sess.CompletionTokens),
			sess.Turns,
		))
	}
	if endpoint != "" {
		parts = append(parts, endpoint)
	}
	if focus != "" {
		parts = append(parts, focus)
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += " · " + parts[i]
	}
	return out
}

// FormatCombinedStatusBar renders a single bottom line: left metrics/activity,
// separator, and right-aligned progress (spinner / tps / done).
func FormatCombinedStatusBar(left, right string, width int) string {
	right = strings.TrimSpace(right)
	if right == "" {
		return left
	}
	if width < 8 {
		return left + " | " + right
	}
	sep := " | "
	rightW := runewidth.StringWidth(right)
	sepW := runewidth.StringWidth(sep)
	leftW := runewidth.StringWidth(left)
	need := leftW + sepW + rightW
	if need <= width {
		pad := width - need
		return left + strings.Repeat(" ", pad) + sep + right
	}
	maxLeftW := width - sepW - rightW
	if maxLeftW < 1 {
		return right
	}
	return runewidth.Truncate(left, maxLeftW, "…") + sep + right
}

func formatTok(prefix string, n int, exact bool) string {
	if exact {
		return fmt.Sprintf("%s%d", prefix, n)
	}
	return fmt.Sprintf("%s~%d", prefix, n)
}

func formatElapsed(d time.Duration) string {
	if d <= 0 {
		return "0.0s"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func formatCompact(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 10_000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
