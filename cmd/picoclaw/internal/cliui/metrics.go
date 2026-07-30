package cliui

import (
	"fmt"
	"time"
)

// TurnMetrics holds per request/response token and timing stats.
type TurnMetrics struct {
	PromptTokens     int
	CompletionTokens int
	PromptExact      bool // true when from provider usage
	CompletionExact  bool
	Elapsed          time.Duration
	Streaming        bool
	TTFT             time.Duration // time to first token; 0 if unknown
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
// Example: "✓ ↑1234 ↓567 · 2.3s · 45.2 tps" or with ~ for estimates.
func FormatTurnStatus(m TurnMetrics) string {
	marker := "✓"
	if m.Streaming {
		marker = "⏳"
	}
	up := formatTok("↑", m.PromptTokens, m.PromptExact)
	down := formatTok("↓", m.CompletionTokens, m.CompletionExact)
	elapsed := formatElapsed(m.Elapsed)
	tps := TPS(m.CompletionTokens, m.Elapsed)
	line := fmt.Sprintf("%s %s %s · %s · %.1f tps", marker, up, down, elapsed, tps)
	if m.TTFT > 0 && !m.Streaming {
		line += fmt.Sprintf(" · ttft %s", formatElapsed(m.TTFT))
	}
	return line
}

// FormatStatusBar builds the bottom overall status line.
func FormatStatusBar(last TurnMetrics, sess SessionMetrics, focus string) string {
	parts := []string{FormatTurnStatus(last)}
	if sess.Turns > 0 {
		parts = append(parts, fmt.Sprintf(
			"Σ ↑%s ↓%s · %d turns",
			formatCompact(sess.PromptTokens),
			formatCompact(sess.CompletionTokens),
			sess.Turns,
		))
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
