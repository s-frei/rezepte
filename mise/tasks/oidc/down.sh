#!/usr/bin/env bash
#MISE description="Stop the test OIDC provider (Dex) for this worktree"
set -euo pipefail
docker compose -f docker-compose.test.yaml down
