# CLI markdown rendering (mdansi) — A/B CPU/RAM

Branch: `feat/glamour-cli-render`

Agent CLI replies (`picoclaw agent`) are rendered with a custom
[goldmark](https://github.com/yuin/goldmark) ANSI/`gotui` NodeRenderer
(`cmd/picoclaw/internal/cliui/mdansi`). Disable with `PICOCLAW_MARKDOWN=0`
(plain text fallback). Deprecated alias: `PICOCLAW_GLAMOUR=0`.

Interactive TUI uses [gotui](https://github.com/metaspartan/gotui) markup
(`RenderGotui`); one-shot / stdout uses ANSI SGR (`Render`).

## How to reproduce

```bash
# Microbenchmarks (plain vs mdansi on the same sample markdown)
go test ./cmd/picoclaw/internal/cliui/ \
  -bench='BenchmarkRenderMarkdown|BenchmarkPrintAgentResponse' \
  -benchmem -count=5

# Stripped binary size + version RSS
CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /tmp/picoclaw ./cmd/picoclaw
/usr/bin/time -v /tmp/picoclaw version
```

Host for numbers below: Linux amd64, AMD Ryzen 7 5800H, Go toolchain as in `go.mod`.

## Microbenchmark (median-ish from count=3)

Sample ~335 bytes of markdown (headings, list, code fence, table, link).

| Path | ns/op | B/op | allocs/op |
|------|------:|-----:|----------:|
| **A — plain** (`PICOCLAW_MARKDOWN=0`) `RenderMarkdown` | ~1.7k | 0 | 0 |
| **B — mdansi** `RenderMarkdown` | ~420k | ~48 KB | ~408 |
| **A — plain** `PrintAgentResponse` | ~2.8k | 32 | 2 |
| **B — mdansi** `PrintAgentResponse` | ~450k | ~47 KB | ~410 |

Compared to the previous **glamour** path on this branch (~310k ns, ~340 KB, ~1805 allocs):
mdansi is in a similar latency ballpark (~0.4 ms/render) but uses **~7× less memory**
and **~4× fewer allocs** per call, and does **not** pull chroma/style assets into the binary.

## Process / binary (stripped `-s -w`)

| Metric | A (main / no heavy MD) | B (mdansi + gotui, this branch) | Prior glamour branch |
|--------|-----------------------:|--------------------------------:|---------------------:|
| Binary size | ~40 MB | **41 MB** | ~81 MB |
| `version` max RSS | ~30 MB | **~32 MB** | ~52 MB |
| `version` wall | ~0.01–0.08 s | ~0.08 s | ~0.02 s |

Dropping glamour recovers ~40 MB of binary size while keeping terminal markdown styling.

## Notes

- Width follows `cliui.InnerWidth()` (or pane column width).
- Failures / disabled env fall back to plain text.
- gotui result pane uses `mdansi.RenderGotui` (termui-style `[text](fg:…)` markup).
