package cliui

import "strings"

// HelpShortcuts is the Ctrl+H / F1 overlay body (left-aligned, glanceable).
const HelpShortcuts = `Global
  Ctrl+H  F1      this help
  Tab     S-Tab   cycle focus · input ↔ result ↔ sessions
  Ctrl+N          new session (keeps prior in tree)
  F2              cycle model / API (status shows model · host)
  Esc     Ctrl+C  quit  (Esc closes this help first)

Input
  Enter           send request
  Ctrl+J          newline
  ↑↓←→            move cursor

Result
  j k  ↑↓         scroll (│/█ bar when overflow)
  PgUp PgDn       page
  g G             top / bottom
  drag-select     copy selection to clipboard

Sessions list
  j k  ↑↓         move · preview ↑/↓ in result
  Enter  l  →     open / activate (never closes on click)
  h  ←            collapse node
  n               new session
  hover ↑↓        thin-box highlight + scroll to turn

List colors
  navy session · lightgreen ↑ · indigo ↓ · yellow cursor

Tree rules
  ● session → ↑ request → ↓ response → ↑ …
  select ↑ to autofill · resend forks a sibling
  select ↓ then send nests under that response`

// HelpOverlaySize returns preferred outer width/height for the help popup.
func HelpOverlaySize(termW, termH int) (w, h int) {
	lines := strings.Split(HelpShortcuts, "\n")
	maxLine := 0
	for _, ln := range lines {
		if n := len([]rune(ln)); n > maxLine {
			maxLine = n
		}
	}
	// borders + padding
	w = maxLine + 4
	h = len(lines) + 4 // title row + borders + Close hint row
	if w < 40 {
		w = 40
	}
	if w > termW-2 {
		w = maxInt(20, termW-2)
	}
	if h > termH-2 {
		h = maxInt(8, termH-2)
	}
	return w, h
}

// CenterRect places a w×h box in the middle of a termW×termH terminal.
func CenterRect(termW, termH, w, h int) Rect {
	if w > termW {
		w = termW
	}
	if h > termH {
		h = termH
	}
	return Rect{
		X: (termW - w) / 2,
		Y: (termH - h) / 2,
		W: w,
		H: h,
	}
}
