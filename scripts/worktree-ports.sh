#!/usr/bin/env bash
# Prints this worktree's port map. Run it as `mise run ports` so the numbers
# come from the root mise.toml [env] block - the one place the base ports and
# the offset arithmetic live.
set -euo pipefail

: "${RZP_BACKEND_PORT:?run this as 'mise run ports'}"

printf 'port offset %s\n' "${REZEPTE_PORT_OFFSET:-0}"
printf '  backend      %s\n' "$RZP_BACKEND_PORT"
printf '  frontend     %s\n' "$RZP_FRONTEND_PORT"
printf '  demo         %s\n' "$RZP_DEMO_PORT"
printf '  docs memory  %s\n' "$RZP_DOCS_MEMORY_PORT"
printf '  docs user    %s\n' "$RZP_DOCS_USER_PORT"
