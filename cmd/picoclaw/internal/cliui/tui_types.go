package cliui

// KeyAction is the high-level outcome of an interactive TUI key handler.
type KeyAction int

const (
	KeyActionNone KeyAction = iota
	KeyActionSubmit
	KeyActionSwitchSession
	KeyActionNewSession
)

// PaneEvent is a user action from the interactive agent TUI.
type PaneEvent struct {
	Action  KeyAction
	Payload string
}
