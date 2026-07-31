package cliui

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ActivityPhase is a long-running agent background phase shown in the status pane.
type ActivityPhase string

const (
	ActivityIdle     ActivityPhase = ""
	ActivityLLM      ActivityPhase = "llm"
	ActivityTool     ActivityPhase = "tool"
	ActivityCompress ActivityPhase = "compress"
	ActivitySubagent ActivityPhase = "subagent"
)

// FormatActivityLine renders an in-flight background activity for the status bar.
// Empty when idle. tick drives the spinner glyph.
func FormatActivityLine(phase ActivityPhase, detail string, started time.Time, tick int, now time.Time) string {
	if phase == ActivityIdle && strings.TrimSpace(detail) == "" {
		return ""
	}
	if now.IsZero() {
		now = time.Now()
	}
	elapsed := time.Duration(0)
	if !started.IsZero() {
		elapsed = now.Sub(started)
		if elapsed < 0 {
			elapsed = 0
		}
	}
	label := activityLabel(phase, detail)
	if label == "" {
		return ""
	}
	return fmt.Sprintf("%s %s · %s", SpinnerFrame(tick), label, formatElapsed(elapsed))
}

func activityLabel(phase ActivityPhase, detail string) string {
	detail = strings.TrimSpace(detail)
	switch phase {
	case ActivityLLM:
		if detail != "" {
			return "llm " + detail
		}
		return "llm call"
	case ActivityTool:
		if detail != "" {
			return "tool " + detail
		}
		return "tool"
	case ActivityCompress:
		return "compress"
	case ActivitySubagent:
		if detail != "" {
			return "subagent " + detail
		}
		return "subagent"
	default:
		return detail
	}
}

// FormatActiveTools builds a compact tool detail from concurrent tool names.
// One tool → its name; several → "name +N".
func FormatActiveTools(tools map[string]int) string {
	if len(tools) == 0 {
		return ""
	}
	names := make([]string, 0, len(tools))
	total := 0
	for name, n := range tools {
		if n <= 0 || name == "" {
			continue
		}
		names = append(names, name)
		total += n
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	if total == 1 {
		return names[0]
	}
	return fmt.Sprintf("%s +%d", names[0], total-1)
}
