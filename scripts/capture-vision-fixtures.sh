#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURES="$ROOT/docs/evaluation/fixtures"
HTML="$FIXTURES/fixture.html"
OUT="$FIXTURES/images"

if [[ "$(uname -s)" == "Darwin" ]]; then
  CHROME="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
else
  CHROME="${CHROME:-$(command -v google-chrome || command -v chromium || true)}"
fi

if [[ ! -x "$CHROME" ]]; then
  echo "Chrome/Chromium not found; set CHROME to its executable path." >&2
  exit 1
fi

mkdir -p "$OUT"

cases=(
  registration-error
  duplicate-submit
  mobile-overlap
  stuck-loader
  not-found-navigation
  checkout-format
  dark-contrast
  stale-status
  upload-stuck
  table-clipping
)

for case_id in "${cases[@]}"; do
  for frame in before after; do
    target="$OUT/$case_id-$frame.png"
    "$CHROME" \
      --headless=new \
      --disable-gpu \
      --hide-scrollbars \
      --window-size=1200,720 \
      --screenshot="$target" \
      "file://$HTML?case=$case_id&frame=$frame" >/dev/null 2>&1
    echo "captured ${target#$ROOT/}"
  done
done
