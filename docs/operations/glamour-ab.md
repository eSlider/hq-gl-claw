# CLI markdown rendering (glamour) — A/B CPU/RAM

Branch: `feat/glamour-cli-render`

Agent CLI replies (`picoclaw agent`) are rendered with
[charmbracelet/glamour](https://github.com/charmbracelet/glamour). Disable with
`PICOCLAW_GLAMOUR=0` (plain text fallback). `--no-color` / non-TTY uses the
`notty` style.

## How to reproduce

```bash
# Microbenchmarks (plain vs glamour on the same sample markdown)
go test ./cmd/picoclaw/internal/cliui/ \
  -bench='BenchmarkRenderMarkdown|BenchmarkPrintAgentResponse' \
  -benchmem -count=5

# Stripped binary size + version RSS
CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /tmp/picoclaw ./cmd/picoclaw
/usr/bin/time -v /tmp/picoclaw version
```

Host for numbers below: Linux amd64, AMD Ryzen 7 5800H, Go toolchain as in `go.mod`.

## Microbenchmark (median-ish from count=5)

Sample ~336 bytes of markdown (headings, list, code fence, table, link).

| Path | ns/op | B/op | allocs/op |
|------|------:|-----:|----------:|
| **A — plain** (`PICOCLAW_GLAMOUR=0`) `RenderMarkdown` | ~35 | 0 | 0 |
| **B — glamour** `RenderMarkdown` | ~310 000 | ~340 KB | ~1805 |
| **A — plain** `PrintAgentResponse` | ~159 | 32 | 2 |
| **B — glamour** `PrintAgentResponse` | ~310 000 | ~330 KB | ~1808 |

Glamour is ~**9 000×** slower per render than a pass-through string and allocates
~0.3 MB per call. Absolute cost is still ~0.3 ms/render — fine for interactive
CLI replies, not free on tiny boards if replies stream continuously.

## Process / binary (stripped `-s -w`)

| Metric | A (main, no glamour) | B (this branch) | Δ |
|--------|---------------------:|----------------:|--:|
| Binary size | 40 MB | 81 MB | +41 MB (~2×) |
| `version` max RSS | ~30 MB | ~52 MB | +22 MB |
| `version` wall | ~0.01 s | ~0.02 s | negligible |

Binary growth is dominated by chroma/goldmark style assets linked with glamour,
even when `PICOCLAW_GLAMOUR=0` at runtime.

## Notes

- Rendering is cached per (style, width); width follows `cliui.InnerWidth()`.
- Failures fall back to plain text.
- For ultra-low RAM targets, keep builds without this dependency or gate with a
  build tag in a follow-up if needed.
