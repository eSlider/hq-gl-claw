package cliui

// Rect is a terminal rectangle in cell coordinates.
type Rect struct {
	X, Y, W, H int
}

// Chrome holds layout rectangles for the interactive agent TUI.
// Top: result | sessions. Bottom: one full-width input pane whose
// title carries status metrics/activity and right-aligned progress.
type Chrome struct {
	Result   Rect
	Sessions Rect
	Input    Rect
}

const (
	sessionsMinW = 18
	sessionsMaxW = 28
	minResultH   = 3
)

// ComputeChrome derives pane geometry from terminal size and input row count.
func ComputeChrome(width, height, inputRows int) Chrome {
	if width < 40 {
		width = 40
	}
	if height < 6 {
		height = 6
	}
	if inputRows < 1 {
		inputRows = 1
	}

	sessW := width / 4
	if sessW < sessionsMinW {
		sessW = sessionsMinW
	}
	if sessW > sessionsMaxW {
		sessW = sessionsMaxW
	}
	if sessW >= width {
		sessW = width / 3
	}
	resultW := width - sessW

	// Prefer keeping inputRows; shrink result if needed, never below minResultH.
	resultH := height - inputRows
	if resultH < minResultH {
		inputRows = height - minResultH
		if inputRows < 1 {
			inputRows = 1
		}
		resultH = height - inputRows
		if resultH < minResultH {
			resultH = minResultH
		}
	}

	return Chrome{
		Result:   Rect{X: 0, Y: 0, W: resultW, H: resultH},
		Sessions: Rect{X: resultW, Y: 0, W: sessW, H: resultH},
		Input:    Rect{X: 0, Y: resultH, W: width, H: inputRows},
	}
}
