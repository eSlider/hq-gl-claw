package cliui

import (
	"strings"
	"unicode"
)

// eventRune extracts a single printable input rune from a gotui event ID.
// Cyrillic and other multi-byte UTF-8 characters are accepted; named keys
// like "<Enter>" are rejected. Do not use len(id)==1 — that is byte length.
func eventRune(id string) (rune, bool) {
	if id == "" || strings.HasPrefix(id, "<") {
		return 0, false
	}
	rs := []rune(id)
	if len(rs) != 1 {
		return 0, false
	}
	r := rs[0]
	if !unicode.IsPrint(r) || unicode.IsControl(r) {
		return 0, false
	}
	return r, true
}
