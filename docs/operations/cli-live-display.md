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
┌────────────────────────────────────┬──────────────────┐
│ top: progress / focus              │ sessions         │
├────────────────────────────────────┤ ● last request…  │
│ result (scrollable, searchable)    │   previous…      │
│ …                                  │   previous…      │
├────────────────────────────────────┤                  │
│ > You: input                       │                  │
├────────────────────────────────────┴──────────────────┤
│ ✓ ↑1234 ↓567 · 2.30s · 45.2 tps · Σ … · 3 turns       │
└───────────────────────────────────────────────────────┘
```

Right panel: current session (●) first, then previous `cli:*` sessions.
Row title = **last user request**, truncated.

Result pane: final answers are **glamour-styled** markdown (`PICOCLAW_GLAMOUR=0` to disable);
streaming shows plain text until the turn completes.

Bottom status: last turn ↑sent / ↓received, wall time, completion tps,
optional ttft, plus session Σ totals.

### Keys

| Key | Action |
|-----|--------|
| Tab / Shift+Tab | Cycle focus: input ↔ result ↔ sessions |
| Enter | Submit (input) / open session (sessions) |
| j/k ↑↓ | Scroll result **or** move session cursor |
| g / G | Top / bottom (result or sessions) |
| n | Next search match (result) / **new session** (sessions) |
| `/` … Enter, N | Vim-like search in result |
| Esc | Clear search / leave sessions → input |
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
