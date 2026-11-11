#!/usr/bin/env bash
#MISE description="Start the test SMTP server (Mailpit) for this worktree"
set -euo pipefail
: "${RZP_MAILPIT_UI_PORT:?run this through mise: mise run mail:up}"
# shellcheck source=mise/lib/instance.sh
. mise/lib/instance.sh
docker compose -f docker-compose.test.yaml up -d mailpit
if ! rzp_wait_for rzp_mailpit_up; then
	echo "mail:up: Mailpit did not answer on http://localhost:$RZP_MAILPIT_UI_PORT within about 10s; see docker compose -f docker-compose.test.yaml logs mailpit" >&2
	exit 1
fi
echo "mail:up: SMTP localhost:$RZP_MAILPIT_SMTP_PORT (security none, any login), inbox http://localhost:$RZP_MAILPIT_UI_PORT"
