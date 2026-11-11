#!/usr/bin/env bash
#MISE description="Start the test OIDC provider (Dex) for this worktree"
set -euo pipefail
: "${RZP_DEX_PORT:?run this through mise: mise run oidc:up}"
# shellcheck source=mise/lib/instance.sh
. mise/lib/instance.sh
mkdir -p .oidc
sed -e "s/\${RZP_DEX_PORT}/$RZP_DEX_PORT/g" \
    -e "s/\${RZP_BACKEND_PORT}/$RZP_BACKEND_PORT/g" \
    -e "s/\${RZP_DEMO_PORT}/$RZP_DEMO_PORT/g" \
    -e "s/\${RZP_E2E_PORT}/$RZP_E2E_PORT/g" \
    -e "s/\${RZP_SCREENSHOTS_PORT}/$RZP_SCREENSHOTS_PORT/g" \
    mise/oidc/dex.yaml.tmpl > .oidc/dex.yaml
# Not `up -d --wait`: the Dex image declares no HEALTHCHECK, so --wait would
# only confirm the container is running, not that Dex is answering yet - poll
# discovery instead, the same document oidc.Login's own discover() reads.
docker compose -f docker-compose.test.yaml up -d dex
if ! rzp_wait_for rzp_dex_up; then
	echo "oidc:up: Dex did not answer on http://localhost:$RZP_DEX_PORT/dex within about 10s; see docker compose -f docker-compose.test.yaml logs" >&2
	exit 1
fi
echo "oidc:up: issuer http://localhost:$RZP_DEX_PORT/dex - Dex: sign in as demo@, mila@ or jonas@example.com, password <name>1234"
echo "  REZEPTE_PUBLIC_URL=http://localhost:$RZP_BACKEND_PORT REZEPTE_OIDC_ISSUER=http://localhost:$RZP_DEX_PORT/dex REZEPTE_OIDC_CLIENT_ID=rezepte REZEPTE_OIDC_CLIENT_SECRET=rezepte-test-secret REZEPTE_OIDC_NAME=Dex"
