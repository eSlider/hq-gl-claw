# CLI markdown / TUI — A/B CPU/RAM (plain → glamour → mdansi+gotui)

Branch: `feat/glamour-cli-render`

This document keeps **both** generations of A/B numbers from this PR so
reviewers can compare the Charm-style path with the current stack.

| Generation | Markdown | Interactive TUI | Status on this branch |
|------------|----------|-----------------|------------------------|
| **A — plain** | pass-through | readline / no styling | always available (`PICOCLAW_MARKDOWN=0`) |
| **B — previous** | [glamour](https://github.com/charmbracelet/glamour) | hand-rolled raw ANSI panes (Charm-oriented; [Bubble Tea](https://github.com/charmbracelet/bubbletea) was considered as the pane rewrite but **not shipped**) | superseded — numbers frozen below |
| **C — current** | custom goldmark **mdansi** | [gotui](https://github.com/metaspartan/gotui) widgets | **this tip** |

Disable styling: `PICOCLAW_MARKDOWN=0` (deprecated alias `PICOCLAW_GLAMOUR=0`).
Disable TUI: `PICOCLAW_PANES=0` (readline).

Host for all numbers: Linux amd64, AMD Ryzen 7 5800H, Go as in `go.mod` at
measurement time. Sample markdown ~335–336 bytes (headings, list, code fence,
table, link).

## How to reproduce (current tip = C)

```bash
go test ./cmd/picoclaw/internal/cliui/ \
  -bench='BenchmarkRenderMarkdown|BenchmarkPrintAgentResponse' \
  -benchmem -count=5

CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /tmp/picoclaw ./cmd/picoclaw
/usr/bin/time -v /tmp/picoclaw version
```

Generation **B** benches are no longer runnable on tip (glamour removed);
figures below are from the earlier glamour commits on this same PR.

---

## 1. Microbenchmark — markdown render

### B — previous PR (glamour), count≈5

| Path | ns/op | B/op | allocs/op |
|------|------:|-----:|----------:|
| **A — plain** `RenderMarkdown` | ~35 | 0 | 0 |
| **B — glamour** `RenderMarkdown` | ~310 000 | ~340 KB | ~1805 |
| **A — plain** `PrintAgentResponse` | ~159 | 32 | 2 |
| **B — glamour** `PrintAgentResponse` | ~310 000 | ~330 KB | ~1808 |

Glamour ≈ **9000×** slower than pass-through on that run; absolute cost ≈ **0.3 ms/render**.
Heavy cost is chroma/style assets (~0.3 MB allocs per call).

### C — current tip (mdansi), count=3

| Path | ns/op | B/op | allocs/op |
|------|------:|-----:|----------:|
| **A — plain** `RenderMarkdown` | ~1.7k | 0 | 0 |
| **C — mdansi** `RenderMarkdown` | ~420k | ~48 KB | ~408 |
| **A — plain** `PrintAgentResponse` | ~2.8k | 32 | 2 |
| **C — mdansi** `PrintAgentResponse` | ~450k | ~47 KB | ~410 |

(Plain ns/op differs across measurement days / Go toolchain noise; use B/op and
allocs/op for cross-generation comparison.)

### Side-by-side (styled path)

| | **B glamour** | **C mdansi** | Δ (C vs B) |
|--|-------------:|-------------:|-----------:|
| ns/op (RenderMarkdown) | ~310k | ~420k | similar (~0.3–0.4 ms) |
| B/op | ~340 KB | ~48 KB | **~7× less** |
| allocs/op | ~1805 | ~408 | **~4× fewer** |

---

## 2. Process / binary (stripped `-s -w`)

| Metric | A (no heavy MD) | **B glamour** (prev PR) | **C mdansi + gotui** (tip) |
|--------|----------------:|------------------------:|---------------------------:|
| Binary size | ~40 MB | ~81 MB (+41 MB, ~2×) | **~41 MB** |
| `version` max RSS | ~30 MB | ~52 MB (+22 MB) | **~32 MB** |
| `version` wall | ~0.01 s | ~0.02 s | ~0.08 s |

Glamour linked chroma/goldmark style assets even when `PICOCLAW_GLAMOUR=0` at
runtime. mdansi + gotui restores near-baseline binary/RSS while keeping styled
replies and a real TUI.

---

## 3. TUI stack note (Bubble Tea vs gotui)

| Approach | Role on this branch |
|----------|---------------------|
| Raw ANSI panes + glamour | Shipped earlier on the PR; layout/cursor bugs |
| Bubble Tea + bubbles (+ glamour) | Planned Charm rewrite; **not landed** — replaced before merge |
| **gotui + mdansi** | Current interactive agent UI |

If evaluating “Charm ecosystem” cost, use **generation B** (glamour) numbers
above for markdown/binary; pane UX regressions are why B was replaced by C.

## Notes

- Width follows `cliui.InnerWidth()` (or pane column width).
- Failures / disabled env fall back to plain text.
- gotui result pane uses `mdansi.RenderGotui` (termui-style `[text](fg:…)` markup);
  one-shot `-m` / stdout uses ANSI SGR (`mdansi.Render`).
- Fenced code blocks use **internal lite lexers** (html/css/js/go/json/shell/yaml):
  language header + `│` gutter + token colors. No Chroma (keeps binary/RSS near
  baseline). Unknown langs stay monochrome framed. Sample HTML-fence render
  (~10 lines): ~33 µs, ~41 KB, ~273 allocs — well under prior glamour ~340 KB/op.
