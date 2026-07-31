# CLI live display & gotui panes

## One-shot (`picoclaw agent -m '…'`)

1. Progress on stderr while waiting
2. Streamed tokens to stdout
3. Footer: `✓ ↑in ↓out · elapsed · tps · ttft …`
   - Exact counts when the provider reports usage (`SetTurnUsage`)
   - `↑~` / `↓~` when estimated (`ceil(runes/4)`)
4. Final reply styled via **mdansi** (goldmark → ANSI)

## Interactive TUI (TTY) — gotui

Built on [metaspartan/gotui](https://github.com/metaspartan/gotui) (tcell widgets).
Markdown in the result pane uses `mdansi.RenderGotui` after each turn completes;
streaming stays plain text.

```
┌────────────────────────────────────┬──────────────────┐
│ stats: 🦞 thinking… / ✨ tps       │ sessions tree    │
├────────────────────────────────────┤ ▼ ● last ask…    │
│ result (scrollable)                │     ↑ request    │
│ …                                  │     ↓ response   │
│                                    │ ▶ ● older…       │
├────────────────────────────────────┤                  │
│ input (TextArea, UTF-8 / Cyrillic) │                  │
├────────────────────────────────────┴──────────────────┤
│ ✓ ↑1234 ↓567 · 2.30s · 45.2 tps · Σ … · focus         │
└───────────────────────────────────────────────────────┘
```

### Sessions tree

- **Level 1 — session** (`▼/▶ ● title` = last user request)
- **Level 2 — turn** (`↑` request, `↓` response)
- Current session is expanded by default; click/Enter a leaf to show it in **result**
- Click/Enter another session node to switch; Enter on current toggles expand

### Launch / session restore

Without `-s`, restore **last used** key from `~/.picoclaw/last_cli_session`,
else newest `cli:<nano>`, else `cli:default`. Force with `-s KEY`.

### Keys & mouse

| Input | Action |
|-------|--------|
| Tab / Shift+Tab | Cycle focus: input ↔ result ↔ sessions |
| Mouse click | Focus pane; in tree, select + activate row |
| Mouse wheel | Scroll result or sessions under cursor |
| Enter / Space / l | Activate tree row (switch / show req\|resp) |
| h / Left | Collapse session node |
| j/k ↑↓ | Move cursor / scroll in focused pane |
| Ctrl+J | Newline in input |
| Ctrl+N | **New session** (any focus) |
| n | **New session** (sessions focus) |
| Esc / Ctrl+C | Quit |
| `exit` / `quit` | Quit from input |

Waiting for a reply: **emoji** animation in stats (`🦞💭✨🔮…`), then
`✨ streaming · N tps`, then `✅ done`.

Bottom status: last turn ↑sent / ↓received, wall time, completion tps,
optional ttft, session Σ, focus name.

Resize re-lays out via `ComputeChrome` + gotui `Clear`/`Render`.

## Env

| Env | Effect |
|-----|--------|
| `PICOCLAW_PANES=0` | Classic readline instead of gotui TUI |
| `PICOCLAW_MARKDOWN=0` | Disable mdansi (plain text) |
| `PICOCLAW_GLAMOUR=0` | Deprecated alias of `PICOCLAW_MARKDOWN=0` |

## Try it

```bash
go build -o ~/.local/bin/picoclaw ./cmd/picoclaw
picoclaw agent                          # interactive gotui TUI
picoclaw agent -m 'Reply with a markdown list'
PICOCLAW_PANES=0 picoclaw agent         # readline fallback
PICOCLAW_MARKDOWN=0 picoclaw agent -m '…'
```

## A/B

See [mdansi-ab.md](mdansi-ab.md) for **plain → glamour (previous) → mdansi+gotui (current)**
benches, binary/RSS, and the Bubble Tea note (planned Charm rewrite, not shipped).

## Tests

```bash
go test -race ./cmd/picoclaw/internal/cliui/... ./cmd/picoclaw/internal/agent/
```

CI: `.github/workflows/cliui.yml` (+ `pr.yml`).
