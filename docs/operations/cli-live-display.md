# CLI live display (progress · tps · streaming)

Agent CLI (`picoclaw agent`) now shows:

1. **Progress bar** on stderr while waiting for the first token (`[==>   ] Thinking…`)
2. **Token streaming** — deltas printed as they arrive (provider streaming when
   enabled; otherwise a rune typewriter fallback)
3. **tps footer** — `42 tok · 28.5 tps` (tokens ≈ ceil(runes/4))

## Controls

| Env | Effect |
|-----|--------|
| `PICOCLAW_GLAMOUR=0` | Disable markdown glamour (batch path / helpers) |
| Model `streaming.enabled` | Opt-in provider streaming (CLI enables for default model) |

## Tests

```bash
go test -race ./cmd/picoclaw/internal/cliui/ ./cmd/picoclaw/internal/agent/
```

CI: `.github/workflows/cliui.yml` (plus existing `pr.yml`).
