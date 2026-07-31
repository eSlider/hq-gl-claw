# CLI live display & gotui panes

## One-shot (`picoclaw agent -m '…'`)

1. Progress bar on stderr while waiting
2. Streamed tokens to stdout
3. Footer: `✓ ↑in ↓out · elapsed · tps · ttft …`
   - Exact counts when the provider reports usage (`SetTurnUsage`)
   - `↑~` / `↓~` when estimated (`ceil(runes/4)`)
4. Final reply styled via **mdansi** (goldmark → ANSI)

## Interactive TUI (TTY) — gotui

Built on [metaspartan/gotui](https://github.com/metaspartan/gotui) (tcell Flex/widgets).
Markdown in the result pane uses `mdansi.RenderGotui` after each turn completes;
streaming stays plain text.

```
┌────────────────────────────────────┬──────────────────┐
│ top: progress / focus              │ sessions         │
├────────────────────────────────────┤ ● last request…  │
│ result (scrollable)                │   previous…      │
│ …                                  │   previous…      │
├────────────────────────────────────┤                  │
│ input (TextArea, native caret)     │                  │
├────────────────────────────────────┴──────────────────┤
│ ✓ ↑1234 ↓567 · 2.30s · 45.2 tps · Σ … · 3 turns       │
└───────────────────────────────────────────────────────┘
```

Right panel: current session (●) first, then previous `cli:*` sessions.
Row title = **last user request**, truncated.

Bottom status: last turn ↑sent / ↓received, wall time, completion tps,
optional ttft, plus session Σ totals.

### Keys

| Key | Action |
|-----|--------|
| Tab / Shift+Tab | Cycle focus: input ↔ result ↔ sessions |
| Enter | Submit (input) / open session (sessions) |
| Ctrl+J | Newline in input |
| j/k ↑↓ / wheel | Scroll result **or** move session cursor |
| g / G / Home / End | Top / bottom of result |
| PgUp / PgDn | Page result |
| n | **New session** (sessions focus) |
| Esc / Ctrl+C | Quit |
| `exit` / `quit` | Quit from input |

Resize clears and re-lays out via `ComputeChrome` + gotui `Render`.

## Env

| Env | Effect |
|-----|--------|
| `PICOCLAW_PANES=0` | Use classic readline instead of gotui TUI |
| `PICOCLAW_MARKDOWN=0` | Disable mdansi (plain text) |
| `PICOCLAW_GLAMOUR=0` | Deprecated alias of `PICOCLAW_MARKDOWN=0` |

## A/B

See [mdansi-ab.md](mdansi-ab.md) for **plain → glamour (previous PR) → mdansi+gotui (current)** benches, binary/RSS, and the Bubble Tea note (planned Charm rewrite, not shipped).

## Tests

```bash
go test -race ./cmd/picoclaw/internal/cliui/... ./cmd/picoclaw/internal/agent/
```

CI: `.github/workflows/cliui.yml` (+ `pr.yml`).
