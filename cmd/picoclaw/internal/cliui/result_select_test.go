package cliui

import (
	"strings"
	"testing"
)

func TestExtractSelection_MultiLine(t *testing.T) {
	lines := []string{"hello world", "second", "third line"}
	got := ExtractSelection(lines, 0, 6, 1, 3)
	if got != "world\nsec" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractSelection_StripsMarkup(t *testing.T) {
	lines := []string{"[hello](fg:red) there"}
	got := ExtractSelection(lines, 0, 0, 0, 5)
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestApplySelectionHighlight(t *testing.T) {
	lines := []string{"abcdef"}
	got := ApplySelectionHighlight(lines, 0, 2, 0, 5)
	if len(got) != 1 || !strings.Contains(got[0], "[cde](fg:black,bg:white)") {
		t.Fatalf("got %q", got)
	}
	if !strings.HasPrefix(got[0], "ab") || !strings.HasSuffix(got[0], "f") {
		t.Fatalf("prefix/suffix lost: %q", got)
	}
}

func TestRuneIndexAtVisualCol(t *testing.T) {
	if got := RuneIndexAtVisualCol("abc", 0); got != 0 {
		t.Fatalf("got %d", got)
	}
	if got := RuneIndexAtVisualCol("abc", 2); got != 2 {
		t.Fatalf("got %d", got)
	}
	if got := RuneIndexAtVisualCol("abc", 99); got != 3 {
		t.Fatalf("got %d", got)
	}
}

func TestTextSelNormalized(t *testing.T) {
	s := textSel{aLine: 2, aCol: 5, bLine: 1, bCol: 3, active: true}
	l0, c0, l1, c1 := s.normalized()
	if l0 != 1 || c0 != 3 || l1 != 2 || c1 != 5 {
		t.Fatalf("got %d:%d %d:%d", l0, c0, l1, c1)
	}
}
