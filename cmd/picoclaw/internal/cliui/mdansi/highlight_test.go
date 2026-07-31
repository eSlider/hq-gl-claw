package mdansi

import (
	"strings"
	"testing"
)

func TestNormalizeLang(t *testing.T) {
	cases := map[string]string{
		"HTML":       "html",
		"htm":        "html",
		"javascript": "js",
		"tsx":        "js",
		"golang":     "go",
		"bash":       "shell",
		"yml":        "yaml",
		"unknown":    "unknown",
	}
	for in, want := range cases {
		if got := NormalizeLang(in); got != want {
			t.Fatalf("NormalizeLang(%q)=%q want %q", in, got, want)
		}
	}
}

func TestHighlight_HTML(t *testing.T) {
	toks := Highlight("html", `<div class="x"><!-- c -->hi</div>`)
	kinds := make([]TokenKind, 0, len(toks))
	var joined strings.Builder
	for _, tok := range toks {
		kinds = append(kinds, tok.Kind)
		joined.WriteString(tok.Text)
	}
	if joined.String() != `<div class="x"><!-- c -->hi</div>` {
		t.Fatalf("text mutated: %q", joined.String())
	}
	has := map[TokenKind]bool{}
	for _, k := range kinds {
		has[k] = true
	}
	for _, want := range []TokenKind{KindTag, KindAttr, KindString, KindComment, KindPlain} {
		if !has[want] {
			t.Fatalf("missing kind %v in %#v", want, toks)
		}
	}
}

func TestHighlight_Go(t *testing.T) {
	toks := Highlight("go", "func main() {\n\treturn nil\n}")
	joined := ""
	hasKw := false
	for _, tok := range toks {
		joined += tok.Text
		if tok.Kind == KindKeyword && (tok.Text == "func" || tok.Text == "return" || tok.Text == "nil") {
			hasKw = true
		}
	}
	if joined != "func main() {\n\treturn nil\n}" {
		t.Fatalf("mutated: %q", joined)
	}
	if !hasKw {
		t.Fatalf("expected keywords: %#v", toks)
	}
}

func TestHighlight_UnknownPlain(t *testing.T) {
	toks := Highlight("rust", "fn main() {}")
	if len(toks) != 1 || toks[0].Kind != KindPlain {
		t.Fatalf("want single plain token, got %#v", toks)
	}
}

func TestRender_HTMLFence(t *testing.T) {
	in := "```html\n<div id=\"a\">x</div>\n```"
	got := Render(in, 80)
	if strings.Contains(got, "```") {
		t.Fatalf("fence markers should be gone: %q", got)
	}
	if !strings.Contains(got, "html") {
		t.Fatalf("expected language header: %q", got)
	}
	if !strings.Contains(got, "│") {
		t.Fatalf("expected gutter: %q", got)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI styling: %q", got)
	}
	if !strings.Contains(stripANSI(got), `<div id="a">x</div>`) {
		t.Fatalf("missing source text: %q", got)
	}
}

func TestRenderGotui_HTMLFence(t *testing.T) {
	in := "```html\n<span class=\"b\">y</span>\n```"
	got := RenderGotui(in, 80)
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("gotui should not emit ANSI: %q", got)
	}
	if !strings.Contains(got, "fg:cyan") && !strings.Contains(got, "[span]") {
		t.Fatalf("expected tag styling: %q", got)
	}
	if !strings.Contains(got, "│") {
		t.Fatalf("expected gutter: %q", got)
	}
}

func TestRender_GoFenceKeywords(t *testing.T) {
	in := "```go\nfunc Hello() string { return \"hi\" }\n```"
	got := Render(in, 80)
	if !strings.Contains(got, "\x1b[36m") { // keyword cyan
		t.Fatalf("expected keyword SGR: %q", got)
	}
	if !strings.Contains(got, "\x1b[32m") { // string green
		t.Fatalf("expected string SGR: %q", got)
	}
}

func TestRender_UnknownFenceMonochrome(t *testing.T) {
	in := "```rust\nfn main() {}\n```"
	got := Render(in, 80)
	if !strings.Contains(got, "rust") {
		t.Fatalf("expected lang header: %q", got)
	}
	plain := stripANSI(got)
	if !strings.Contains(plain, "fn main() {}") {
		t.Fatalf("missing body: %q", got)
	}
}

func BenchmarkHighlight_HTML(b *testing.B) {
	src := strings.Repeat(`<div class="row"><span id="x">hello</span><!-- note --></div>`+"\n", 20)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Highlight("html", src)
	}
}

func BenchmarkRender_HTMLFence(b *testing.B) {
	in := "# Doc\n\n```html\n" + strings.Repeat("<p class=\"c\">hi</p>\n", 10) + "```\n"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Render(in, 80)
	}
}
