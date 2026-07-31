package cliui

// thinkingEmojis cycles while waiting for the first stream token.
var thinkingEmojis = []string{"🦞", "💭", "✨", "🔮", "⚡", "🧠", "🌊", "🔥"}

// EmojiProgress returns an animated thinking frame for tick n.
func EmojiProgress(tick int) string {
	if tick < 0 {
		tick = 0
	}
	e := thinkingEmojis[tick%len(thinkingEmojis)]
	return e + " thinking…"
}
