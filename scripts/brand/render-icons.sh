#!/usr/bin/env bash
# Render the Linux icon set from docs/brand/logo. See render-icons.mjs.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

# Same Playwright install as scripts/capture-screenshots.sh, including the
# temporary workspace file pnpm 10+ needs to allow Playwright's build step.
workspace=""
cleanup() { [[ -n "$workspace" ]] && rm -f "$workspace"; return 0; }
trap cleanup EXIT
major="$(cd /tmp && pnpm --version | cut -d. -f1)"
if [[ "$major" -ge 10 && ! -f scripts/screenshots/pnpm-workspace.yaml ]]; then
  workspace="scripts/screenshots/pnpm-workspace.yaml"
  printf '%s\n' 'packages:' '  - "."' 'allowBuilds:' '  esbuild: true' '  playwright: true' > "$workspace"
fi
(cd scripts/screenshots && pnpm install && pnpm exec playwright install chromium)

node scripts/brand/render-icons.mjs
