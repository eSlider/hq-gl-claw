package cliui

// progressSpinners cycles while waiting / streaming (quarter-circle + related).
var progressSpinners = []string{"◐", "◓", "◑", "◒"}

// SpinnerFrame returns the spinner glyph for tick n.
func SpinnerFrame(tick int) string {
	if tick < 0 {
		tick = 0
	}
	return progressSpinners[tick%len(progressSpinners)]
}

// EmojiProgress returns an animated thinking frame for tick n.
func EmojiProgress(tick int) string {
	return SpinnerFrame(tick) + " thinking…"
}
