#!/usr/bin/env bash
set -euo pipefail
export PATH="/tmp/codex-sshbin:$PATH"
source deploy/lib.sh
ROOT=$(repo_root)
load_local_deploy_env "$ROOT"
printf 'DEPLOYING_HOST=%s\n' "$DEPLOY_HOST"
printf 'DEPLOYING_BASE=%s\n' "$DEPLOY_BASE_DIR"
bash ./deploy/deploy.sh --parallel --parallel-port 18080 --release-note "parallel validation deploy"