package cliui

import ui "github.com/metaspartan/gotui/v5"

// Centralized palette and glyphs for the interactive TUI.
//
// Keeping these in one place lets the whole console share a consistent,
// glanceable visual language: one accent color for "active", one set of
// arrows for request/response, one set of box-drawing connectors for the tree.
var (
	colorAccent   = ui.ColorCyan  // focused pane border / active session
	colorIdle     = ui.ColorWhite // unfocused pane border
	colorProgress = ui.ColorYellow
)

// Tree glyphs. A single glyph per concept avoids duplicated/ambiguous icons.
const (
	glyphSessionActive = "●" // current session marker
	glyphSessionIdle   = "○" // other sessions
	glyphRequest       = "↑" // user request
	glyphResponse      = "↓" // assistant response
	glyphCollapsed     = "▶" // node has hidden children

	connectorMid  = "├" // sibling with more below
	connectorLast = "└" // last sibling
	connectorLine = "─" // expanded / leaf connector fill
)

// Key hints shown in pane titles so controls are discoverable at a glance.
const inputHint = "input · ⏎ send · ^J newline · ^N new"
