package cliui

import "strings"

// ViewWindow returns lines[offset:offset+height] joined by newlines.
func ViewWindow(lines []string, offset, height int) string {
	if height < 1 {
		height = 1
	}
	n := len(lines)
	if n == 0 {
		return ""
	}
	maxOff := n - height
	if maxOff < 0 {
		maxOff = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOff {
		offset = maxOff
	}
	end := offset + height
	if end > n {
		end = n
	}
	return strings.Join(lines[offset:end], "\n")
}

// ClampOffset keeps offset in [0, max(0, total-page)].
func ClampOffset(offset, total, page int) int {
	if page < 1 {
		page = 1
	}
	maxOff := total - page
	if maxOff < 0 {
		maxOff = 0
	}
	if offset < 0 {
		return 0
	}
	if offset > maxOff {
		return maxOff
	}
	return offset
}

// PageDown advances offset by page size.
func PageDown(offset, total, page int) int {
	return ClampOffset(offset+page, total, page)
}

// PageUp retreats offset by page size.
func PageUp(offset, page int) int {
	return ClampOffset(offset-page, offset+page, page)
}
