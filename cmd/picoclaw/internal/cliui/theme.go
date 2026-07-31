package cliui

import ui "github.com/metaspartan/gotui/v5"

// Centralized palette and glyphs for the interactive TUI.
//
// Keeping these in one place lets the whole console share a consistent,
// glanceable visual language: one accent color for "active", one set of
// arrows for request/response, bg colors for the sessions list.
var (
	colorAccent = ui.ColorCyan  // focused pane border / active session
	colorIdle   = ui.ColorWhite // unfocused pane border
)

// Session list glyphs. A single glyph per concept; rows are flat (no tree lines).
const (
	glyphSessionActive = "●" // current session marker
	glyphSessionIdle   = "○" // other sessions
	glyphRequest       = "↑" // user request
	glyphResponse      = "↓" // assistant response
	glyphCollapsed     = "▶" // session has hidden children
	glyphScrollTrack   = "│" // result pane scrollbar track
	glyphScrollThumb   = "█" // result pane scrollbar thumb
)

// gotui markup color names for session-list row backgrounds (fg chosen for contrast).
const (
	listStyleSessionFG  = "white"
	listStyleSessionBG  = "navy"
	listStyleRequestFG  = "black"
	listStyleRequestBG  = "lightgreen"
	listStyleResponseFG = "white"
	listStyleResponseBG = "indigo"
	listStyleSelectFG   = "black"
	listStyleSelectBG   = "yellow"
)

// Key hints shown on the input pane bottom border (status uses the top title).
const inputHint = "⏎ send · ^J newline · ^N new · F2 model · ^H help"
