#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
./node_modules/.bin/vue-tsc -b
./node_modules/.bin/vite build --config vite.oa.config.ts --mode production
