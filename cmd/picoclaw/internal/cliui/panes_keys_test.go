package cliui

import "testing"

func TestParseKeySeqArrowsAndTab(t *testing.T) {
	cases := []struct {
		seq  string
		want Key
	}{
		{"\t", KeyTab},
		{"\x1b[Z", KeyShiftTab},
		{"\x1b[A", KeyUp},
		{"\x1b[B", KeyDown},
		{"\x1b[5~", KeyPageUp},
		{"\x1b[6~", KeyPageDown},
		{"a", KeyRune('a')},
		{"/", KeyRune('/')},
	}
	for _, tc := range cases {
		got, err := ParseKeySeq(tc.seq)
		if err != nil {
			t.Fatalf("seq %q: %v", tc.seq, err)
		}
		if got != tc.want {
			t.Fatalf("seq %q: got %v want %v", tc.seq, got, tc.want)
		}
	}
}
