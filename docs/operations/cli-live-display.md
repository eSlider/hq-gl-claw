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
│ result (scrollable)                │ sessions tree    │
│ …                                  │ ▼ ● last ask…    │
│                                    │     ↑ request    │
│                                    │     ↓ response   │
├────────────────────────────────────┴──────────────────┤
│ input (TextArea, UTF-8 / Cyrillic)                    │
├────────────────────────────────────┬──────────────────┤
│ ✓ ↑in ↓out · tps · Σ · focus       │ 🦞 thinking…     │
└────────────────────────────────────┴──────────────────┘
```

Bottom row: **status** (left, turn/session metrics) shares the corner with **progress**
(right — emoji thinking / streaming tps / done).

### Sessions tree

- **Level 1 — session** (`▼/▶ ● title` = last user request)
- **Level 2 — turn** (`↑` request, `↓` response)
- Current session is expanded by default; click/Enter a session **displays** it
  (always expands — collapse only with `h` / Left)
- Hover (mouse move) or j/k over ↑/↓ **scrolls** the result transcript to that
  block and draws a **thin** (non-rounded) highlight box — does not replace content
- Ctrl+N starts a new session without removing prior sessions from the tree

### Launch / session restore

Without `-s`, restore **last used** key from `~/.picoclaw/last_cli_session`,
else newest `cli:<nano>`, else `cli:default`. Force with `-s KEY`.

### Keys & mouse

| Input | Action |
|-------|--------|
| Tab / Shift+Tab | Cycle focus: input ↔ result ↔ sessions |
| Mouse click | Focus pane; session → display; ↑/↓ → thin-box highlight |
| Mouse move (hover) | Over ↑/↓ → scroll + thin-box highlight in result |
| Mouse wheel | Scroll result or sessions under cursor |
| Enter / Space / l | Activate tree row (display session / highlight req\|resp) |
| h / Left | Collapse session node |
| j/k ↑↓ | Move cursor; preview highlight for ↑/↓ rows |
| Ctrl+J | Newline in input |
| Ctrl+N | **New session** (any focus; keeps prior sessions in tree) |
| n | **New session** (sessions focus) |
| Esc / Ctrl+C | Quit |
| `exit` / `quit` | Quit from input |

Waiting for a reply: **emoji** animation in the bottom-right progress cell
(`🦞💭✨🔮…`) for the **whole turn** (including tool gaps after the first token),
then `✅ done`. Left status keeps ↑↓ metrics with `⏳` while streaming.

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
