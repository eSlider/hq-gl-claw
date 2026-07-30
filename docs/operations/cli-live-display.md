# CLI live display & panes

## One-shot (`picoclaw agent -m '…'`)

1. Progress bar on stderr while waiting
2. Streamed tokens to stdout
3. Footer: `✓ ↑in ↓out · elapsed · tps · ttft …`
   - Exact counts when the provider reports usage (`SetTurnUsage`)
   - `↑~` / `↓~` when estimated (`ceil(runes/4)`)

## Interactive panes (TTY)

Inspired by [go-ollama TUI](https://github.com/eSlider/go-ollama/blob/main/examples/tui/main.go):

```
┌──────────────────────────────────────────────────────────┐
│ top: progress / focus hints                              │
├──────────────────────────────────────────────────────────┤
│ result (scrollable, searchable)                          │
│ …                                                        │
├──────────────────────────────────────────────────────────┤
│ > You: input                                             │
├──────────────────────────────────────────────────────────┤
│ ✓ ↑1234 ↓567 · 2.30s · 45.2 tps · Σ ↑5.2k ↓1.1k · 3 turns│
└──────────────────────────────────────────────────────────┘
```

Bottom status (overall): last turn ↑sent / ↓received, wall time, completion tps,
optional ttft, plus session Σ totals.

### Keys

| Key | Action |
|-----|--------|
| Tab / Shift+Tab | Toggle focus: input ↔ result |
| Enter | Submit (input focus) |
| ↑↓ / j k | Scroll result |
| PgUp / PgDn / u d | Page / half-page |
| g / G | Top / bottom |
| `/` then pattern + Enter | Vim-like search |
| n / N | Next / previous match |
| Esc | Clear search / cancel search entry |
| Ctrl+C | Quit |

Resize (SIGWINCH) rewraps content and redraws the full frame.

## Env

| Env | Effect |
|-----|--------|
| `PICOCLAW_PANES=0` | Use classic readline instead of panes |
| `PICOCLAW_GLAMOUR=0` | Disable glamour markdown helpers |

## Tests

```bash
go test -race ./cmd/picoclaw/internal/cliui/ ./cmd/picoclaw/internal/agent/
```

CI: `.github/workflows/cliui.yml` (+ `pr.yml`).
