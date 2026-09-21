#!/usr/bin/env bash
#MISE description="Build, start the binary on a free port with a temp data dir, run Playwright"
#MISE depends=["//:build"]
#
# The frontend end-to-end suite runs against the production binary, which
# serves the built SPA from one origin. This starts an *empty* instance - no
# --demo - because every spec creates the recipes and users it needs.
# See docs/memory/content/howtos/verify-ui.mdx.
set -euo pipefail
. mise/lib/instance.sh

PORT="${REZEPTE_E2E_PORT:-${RZP_BACKEND_PORT:?run this through mise: mise run e2e}}"
RZP_PORT_HINT="stop that process, or run with REZEPTE_E2E_PORT=<other>"

rzp_require_binary e2e
rzp_require_free_port e2e "$PORT"
rzp_make_data_dir
# -u clears REZEPTE_ADMIN_USER so a developer who exports it still gets the
# `admin` that frontend/e2e/helpers.ts logs in as. The password follows the
# repository's rule for bootstrapped users, username + 1234
# (docs/memory/content/conventions/dev-credentials.mdx).
#
# REZEPTE_LOCALE=de: the instance default is English, but the suite's
# selectors are German (frontend/e2e/helpers.ts pins the PARAGLIDE_LOCALE
# cookie to German before every login). That pin only reaches the
# unauthenticated /login page - a login response sets the cookie from the
# account's own stored locale, and every account this suite creates without
# naming one (admin included) takes this default, so it has to be German too
# or the very first post-login navigation would flip the language back.
env -u REZEPTE_ADMIN_USER \
	REZEPTE_ADDR=":$PORT" REZEPTE_DATA_DIR="$RZP_DATA_DIR" REZEPTE_ADMIN_PASSWORD=admin1234 \
	REZEPTE_LOCALE=de \
	service/bin/rezepte &
PID=$!
trap 'rzp_stop "$PID"' EXIT
rzp_wait_healthz e2e "$PORT" "$PID" 5

cd frontend && E2E_BASE_URL="http://localhost:$PORT" bun run test:e2e
