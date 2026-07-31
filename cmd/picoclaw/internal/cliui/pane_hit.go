package cliui

// HitTestPane returns which focus pane contains (x,y), or Focus(-1) if none.
func HitTestPane(c Chrome, x, y int) Focus {
	in := func(r Rect) bool {
		return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
	}
	// Prefer interactive panes (input over status overlap edge cases).
	if in(c.Input) {
		return FocusInput
	}
	if in(c.Result) {
		return FocusResult
	}
	if in(c.Sessions) {
		return FocusSessions
	}
	return FocusNone
}

// PointInRect reports whether (x,y) lies in r.
func PointInRect(r Rect, x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}
