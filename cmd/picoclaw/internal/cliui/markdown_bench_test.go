package cliui

import (
	"fmt"
	"io"
	"os"
	"testing"
)

const benchMarkdown = `# Hello PicoClaw

Here is a **summary** of the change:

1. Added glamour rendering
2. Kept plain fallback
3. Measured CPU/RAM

## Example

` + "```go\nfunc main() {\n    fmt.Println(\"hi\")\n}\n```" + `

| Metric | Value |
|--------|------:|
| RSS    | 12 MB |
| CPU    | 3.2%  |

> Note: personal experiment only.

Visit https://picoclaw.io for docs.
`

func BenchmarkRenderMarkdown_Plain(b *testing.B) {
	b.Setenv(envGlamourDisable, "0")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderMarkdown(benchMarkdown)
	}
}

func BenchmarkRenderMarkdown_Glamour(b *testing.B) {
	b.Setenv(envGlamourDisable, "1")
	// Warm renderer outside timed loop.
	_ = RenderMarkdown(benchMarkdown)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderMarkdown(benchMarkdown)
	}
}

func BenchmarkPrintAgentResponse_Plain(b *testing.B) {
	b.Setenv(envGlamourDisable, "0")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintAgentResponse(io.Discard, "🦞", benchMarkdown)
	}
}

func BenchmarkPrintAgentResponse_Glamour(b *testing.B) {
	b.Setenv(envGlamourDisable, "1")
	_ = RenderMarkdown(benchMarkdown)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintAgentResponse(io.Discard, "🦞", benchMarkdown)
	}
}

// Ensure sample stays representative if edited.
func TestBenchMarkdownNonEmpty(t *testing.T) {
	if len(benchMarkdown) < 100 {
		t.Fatalf("bench markdown too small: %d", len(benchMarkdown))
	}
	fmt.Fprintln(os.Stderr, "bench markdown bytes:", len(benchMarkdown))
}
