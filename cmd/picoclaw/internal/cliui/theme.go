package cliui

import (
	"github.com/gdamore/tcell/v3"
	ui "github.com/metaspartan/gotui/v5"
)

// Centralized palette and glyphs for the interactive TUI.
//
// Keeping these in one place lets the whole console share a consistent,
// glanceable visual language: one accent color for "active", one set of
// arrows for request/response, 256-color hex backgrounds for the sessions list.
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

// Session-list row colors use hex so gotui/tcell resolve 256-color (or
// truecolor) backgrounds instead of the basic 16 named colors.
// Values match common xterm-256 palette entries for consistent terminals.
const (
	listStyleSessionFG  = "#e8eef8"
	listStyleSessionBG  = "#005f87" // xterm 24
	listStyleRequestFG  = "#e8ffe8"
	listStyleRequestBG  = "#005f00" // xterm 22
	listStyleResponseFG = "#f5e8ff"
	listStyleResponseBG = "#5f0087" // xterm 54
	listStyleSelectFG   = "#1c1c1c"
	listStyleSelectBG   = "#ffd700" // xterm 220
)

var (
	listSelectFg = tcell.GetColor(listStyleSelectFG)
	listSelectBg = tcell.GetColor(listStyleSelectBG)
)

// Key hints shown on the input pane bottom border (status uses the top title).
const inputHint = "⏎ send · ^J newline · ^N new · F2 model · ^H help"
