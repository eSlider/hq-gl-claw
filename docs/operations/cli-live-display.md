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
Markdown in the result pane uses `mdansi.RenderGotui` after each turn completes
and for ↓ response blocks in the session transcript (↑ requests stay plain);
streaming stays plain text.

```
┌───────────────────────────────────┬──────────────────┐
│ result (scrollable)            █ │ sessions tree    │
│ …                              │ │ ▼ ● last ask…    │
│                                │ │     ↑ request    │
│                                │ │     ↓ response   │
├───────────────────────────────────┴──────────────────┤
│ input (TextArea, UTF-8 / Cyrillic)                   │
├───────────────────────────────────┬──────────────────┤
│ ✓ ↑in ↓out · tps · Σ · focus      │ ◐ thinking…      │
└───────────────────────────────────┴──────────────────┘
```

Bottom row: **status** (left — turn metrics + live activity) shares the corner with
**progress** (right — ◐ spinner / streaming tps / done). While a turn runs, status
shows the current background phase (`llm model`, `tool name`, `compress`,
`subagent`) with elapsed time so long tool/API gaps are visible. The result pane
shows a `│`/`█` scrollbar on the right edge when content overflows the viewport.

### Sessions list

Rows are a **flat list** (no tree connectors), colored by kind:

- **session** — white on navy (`●` / `○`)
- **↑ request** — black on light green
- **↓ response** — white on indigo
- **cursor** — black on yellow

Collapse with `h` / Left still hides children (`▶` on the session row).

Nested structure (logic only; display is flat):

```
● session title
↑ request
↓ response
↑ follow-up A
↑ follow-up B   ← branching under a response
```

Rules:

1. **Session selected + Enter in input** → new request leaf under that session
2. **Request selected** → input autofilled; send creates a **sibling** request under the same parent (edit & resend / fork)
3. **Response selected + send** → new request nests **under that response** (many requests per response)
4. Collapse any node with `h` / Left; click/Enter on a session always displays it (expands)

Hover / j·k over ↑/↓ scrolls the result transcript with a thin highlight box.

### Launch / session restore

Without `-s`, restore **last used** key from `~/.picoclaw/last_cli_session`,
else newest `cli:<nano>`, else `cli:default`. Force with `-s KEY`.

### Keys & mouse

| Input | Action |
|-------|--------|
| Ctrl+H / F1 | Toggle help popup with shortcuts |
| Tab / Shift+Tab | Cycle focus: input ↔ result ↔ sessions |
| Mouse click | Focus pane; session → display; ↑/↓ → thin-box highlight |
| Mouse move (hover) | Over ↑/↓ → scroll + thin-box highlight in result; session hover only redraws when the hovered row changes (motion floods are ignored) |
| Mouse drag (result) | Select text → copy to system clipboard |
| Mouse wheel | Scroll result or sessions under cursor |
| Enter / Space / l | Activate tree row (display session / highlight req\|resp) |
| h / Left | Collapse session node |
| j/k ↑↓ | Move cursor; preview highlight for ↑/↓ rows |
| Ctrl+J | Newline in input |
| Ctrl+N | **New session** (any focus; keeps prior sessions in tree) |
| F2 | Cycle enabled model/API (status shows `model · host`) |
| n | **New session** (sessions focus) |
| Esc / Ctrl+C | Quit (Esc first closes help if open) |
| `exit` / `quit` | Quit from input |

Waiting for a reply: **◐◓◑◒** spinner in the bottom-right progress cell
for the **whole turn** (including tool gaps after the first token),
then `● done`. Left status keeps ↑↓ metrics with `⏳` while streaming.

Bottom status: last turn ↑sent / ↓received, wall time, completion tps,
optional ttft, session Σ, **model · api host**, focus name.

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
