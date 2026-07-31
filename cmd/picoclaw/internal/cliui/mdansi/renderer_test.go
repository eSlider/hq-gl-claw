package mdansi

import (
	"strings"
	"testing"
)

func TestRender_Bold(t *testing.T) {
	got := Render("hello **world**", 80)
	if !strings.Contains(got, "world") {
		t.Fatalf("missing text: %q", got)
	}
	if !strings.Contains(got, "\x1b[1m") {
		t.Fatalf("expected bold SGR, got %q", got)
	}
	if !strings.Contains(got, "\x1b[0m") {
		t.Fatalf("expected reset SGR, got %q", got)
	}
}

func TestRender_Italic(t *testing.T) {
	got := Render("say *hi*", 80)
	if !strings.Contains(got, "\x1b[3m") {
		t.Fatalf("expected italic SGR, got %q", got)
	}
	if !strings.Contains(got, "hi") {
		t.Fatalf("missing text: %q", got)
	}
}

func TestRender_InlineCode(t *testing.T) {
	got := Render("use `fmt.Println`", 80)
	if !strings.Contains(got, "fmt.Println") {
		t.Fatalf("missing code text: %q", got)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI styling for code, got %q", got)
	}
}

func TestRender_Heading(t *testing.T) {
	got := Render("# Title", 80)
	if !strings.Contains(got, "Title") {
		t.Fatalf("missing heading: %q", got)
	}
	if !strings.Contains(got, "\x1b[1;") && !strings.Contains(got, "\x1b[1m") {
		// heading uses bold+color
		if !strings.Contains(got, "\x1b[") {
			t.Fatalf("expected styled heading, got %q", got)
		}
	}
}

func TestRender_FencedCode(t *testing.T) {
	in := "```go\nfunc main() {}\n```"
	got := Render(in, 80)
	if !strings.Contains(got, "func main()") {
		t.Fatalf("missing code body: %q", got)
	}
}

func TestRender_Lists(t *testing.T) {
	in := "- one\n- two\n\n1. a\n2. b"
	got := Render(in, 80)
	for _, want := range []string{"one", "two", "a", "b"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if !strings.Contains(got, "•") && !strings.Contains(got, "-") {
		t.Fatalf("expected list marker, got %q", got)
	}
}

func TestRender_WrapWidth(t *testing.T) {
	in := "word1 word2 word3 word4 word5 word6 word7 word8"
	got := Render(in, 20)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapped lines, got %q", got)
	}
	for _, line := range lines {
		plain := stripANSI(line)
		if runewidthString(plain) > 20 {
			t.Fatalf("line wider than 20: %q (vis=%d)", plain, runewidthString(plain))
		}
	}
}

func TestRender_Empty(t *testing.T) {
	if got := Render("", 80); got != "" {
		t.Fatalf("empty input should stay empty, got %q", got)
	}
}

func TestRenderGotui_Heading(t *testing.T) {
	got := RenderGotui("# Title", 80)
	if !strings.Contains(got, "[Title](fg:cyan,mod:bold)") && !strings.Contains(got, "Title") {
		t.Fatalf("expected gotui markup heading, got %q", got)
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("gotui mode should not emit ANSI: %q", got)
	}
}
