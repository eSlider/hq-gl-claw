#!/usr/bin/env bash
# A/B CPU/RAM for CLI markdown rendering (plain vs glamour).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
OUT="${OUT:-/tmp/picoclaw-glamour-ab}"
mkdir -p "$OUT"

echo "== microbenchmarks =="
go test ./cmd/picoclaw/internal/cliui/ \
  -bench='BenchmarkRenderMarkdown|BenchmarkPrintAgentResponse' \
  -benchmem -count="${COUNT:-3}" | tee "$OUT/bench.txt"

echo "== stripped build + version RSS =="
CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o "$OUT/picoclaw" ./cmd/picoclaw
ls -la "$OUT/picoclaw"
/usr/bin/time -v "$OUT/picoclaw" version 2>"$OUT/version-time.txt" >/dev/null || true
grep -E 'Maximum resident|Elapsed|User time|System time' "$OUT/version-time.txt" || true
echo "wrote $OUT"
