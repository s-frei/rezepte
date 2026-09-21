#!/usr/bin/env bash
#MISE description="Print this worktree's port map"
#
# The numbers come from the root mise.toml [env] block, the one place the base
# ports and the offset arithmetic live, which is why this is a task and not a
# script anyone can run: outside mise the variables are simply absent.
set -euo pipefail

: "${RZP_BACKEND_PORT:?run this through mise: mise run ports}"

printf 'port offset %s\n' "${REZEPTE_PORT_OFFSET:-0}"
printf '  backend      %s\n' "$RZP_BACKEND_PORT"
printf '  frontend     %s\n' "$RZP_FRONTEND_PORT"
printf '  demo         %s\n' "$RZP_DEMO_PORT"
printf '  docs memory  %s\n' "$RZP_DOCS_MEMORY_PORT"
printf '  docs user    %s\n' "$RZP_DOCS_USER_PORT"
