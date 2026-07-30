# CLI live display & panes

## One-shot (`picoclaw agent -m '…'`)

1. Progress bar on stderr while waiting
2. Streamed tokens to stdout
3. Footer `N tok · X.X tps`

## Interactive panes (TTY)

When stdin/stdout are a terminal (disable with `PICOCLAW_PANES=0`):

```
┌─────────────────────────────────────────┐
│ stats (1 line: tps / progress / focus)  │
├─────────────────────────────────────────┤
│ result (scrollable, searchable)         │
│ …                                       │
├─────────────────────────────────────────┤
│ > You: input                            │
└─────────────────────────────────────────┘
```

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
