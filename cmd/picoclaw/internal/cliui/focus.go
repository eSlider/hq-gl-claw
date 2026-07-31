package cliui

// Focus identifies which pane receives keyboard input.
type Focus int

const (
	FocusInput Focus = iota
	FocusResult
	FocusSessions
	FocusSearch // legacy search mode; not in Tab cycle
)

// FocusNone means no pane hit (mouse outside interactive regions).
const FocusNone Focus = -1

func (f Focus) Next() Focus {
	switch f {
	case FocusInput:
		return FocusResult
	case FocusResult:
		return FocusSessions
	case FocusSessions:
		return FocusInput
	default:
		return FocusInput
	}
}

func (f Focus) Prev() Focus {
	switch f {
	case FocusInput:
		return FocusSessions
	case FocusResult:
		return FocusInput
	case FocusSessions:
		return FocusResult
	default:
		return FocusInput
	}
}

func (f Focus) String() string {
	switch f {
	case FocusInput:
		return "input"
	case FocusResult:
		return "result"
	case FocusSessions:
		return "sessions"
	case FocusSearch:
		return "search"
	default:
		return "unknown"
	}
}

// AllowKey reports whether key is accepted for the focused pane.
// While streaming, Enter (submit) on input is blocked; Tab and scroll remain ok.
func AllowKey(focus Focus, streaming bool, key string) bool {
	if !streaming {
		return true
	}
	switch key {
	case "tab", "shift+tab", "j", "k", "up", "down", "pgup", "pgdown", "wheelup", "wheeldown":
		return true
	case "enter":
		return focus != FocusInput
	default:
		return focus == FocusInput || focus == FocusResult || focus == FocusSessions
	}
}
