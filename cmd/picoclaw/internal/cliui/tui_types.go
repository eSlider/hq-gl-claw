package cliui

// KeyAction is the high-level outcome of an interactive TUI key handler.
type KeyAction int

const (
	KeyActionNone KeyAction = iota
	KeyActionSubmit
	KeyActionSwitchSession
	KeyActionNewSession
	KeyActionCycleModel
)

// PaneEvent is a user action from the interactive agent TUI.
type PaneEvent struct {
	Action  KeyAction
	Payload string

	// Submit fields (KeyActionSubmit):
	SessionKey    string        // session to run against
	NodeID        string        // new request node id (for attaching response)
	HistoryBefore []ChatMessage // path messages before the new request (fork/branch)
}
