package cliui

// Rect is a terminal rectangle in cell coordinates.
type Rect struct {
	X, Y, W, H int
}

// Chrome holds layout rectangles for the interactive agent TUI.
type Chrome struct {
	Stats    Rect
	Result   Rect
	Sessions Rect
	Input    Rect
	Status   Rect
}

const (
	sessionsMinW = 18
	sessionsMaxW = 28
	minResultH   = 3
	// gotui Block.SetRect always insets Inner by 1 row/col; bordered
	// widgets need outer H>=3 for one line of text (top+content+bottom).
	statsH  = 3
	statusH = 3
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
	used := statsH + inputRows + statusH
	resultH := height - used
	if resultH < minResultH {
		// Shrink input to fit.
		inputRows = height - statsH - statusH - minResultH
		if inputRows < 1 {
			inputRows = 1
		}
		resultH = height - statsH - inputRows - statusH
		if resultH < minResultH {
			resultH = minResultH
		}
	}

	return Chrome{
		Stats:    Rect{X: 0, Y: 0, W: resultW, H: statsH},
		Result:   Rect{X: 0, Y: statsH, W: resultW, H: resultH},
		Sessions: Rect{X: resultW, Y: 0, W: sessW, H: statsH + resultH},
		Input:    Rect{X: 0, Y: statsH + resultH, W: width, H: inputRows},
		Status:   Rect{X: 0, Y: statsH + resultH + inputRows, W: width, H: statusH},
	}
}
