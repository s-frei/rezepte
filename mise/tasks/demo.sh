#!/usr/bin/env bash
#MISE description="Start rezepte --demo on this worktree's demo port; with arguments, run them against it instead"
#MISE depends=["//:build"]
#
# `mise run demo` starts the binary with demo data and keeps it up until Ctrl-C
# - the fastest way to a realistic instance to click through or point a probe
# at. `mise run demo -- <cmd>` runs one command against it and stops it
# afterwards, which is how the screenshot and OpenAPI tasks get an instance
# they own. See docs/memory/content/features/demo-mode.mdx.
set -euo pipefail
. mise/lib/instance.sh

: "${RZP_DEMO_PORT:?run this through mise: mise run demo}"
PORT="$RZP_DEMO_PORT"

rzp_require_binary demo
rzp_require_free_port demo "$PORT"
rzp_make_data_dir
# -u clears REZEPTE_ADMIN_USER and REZEPTE_ADMIN_PASSWORD from the environment
# so a developer who has them exported still gets the demo/demo1234 credentials
# the screenshot specs and the OpenAPI fetch log in with, instead of whatever
# they last used.
#
# RZP_DEMO_LOCALE, when set, beats REZEPTE_LOCALE: a caller that needs one
# language (the screenshot tasks need English) cannot set REZEPTE_LOCALE
# itself, because this task's own config env - a developer's mise.local.toml
# included - is applied again on the way in and would override it.
env -u REZEPTE_ADMIN_USER -u REZEPTE_ADMIN_PASSWORD \
	REZEPTE_ADDR=":$PORT" REZEPTE_DATA_DIR="$RZP_DATA_DIR" REZEPTE_LOG_LEVEL=warn \
	REZEPTE_LOCALE="${RZP_DEMO_LOCALE:-${REZEPTE_LOCALE:-en}}" \
	service/bin/rezepte --demo &
PID=$!
trap 'rzp_stop "$PID"' EXIT
# Seeding twelve recipes and nine placeholder images takes a moment, and the
# server answers only once it is done - hence 30s rather than the 5s an empty
# instance needs.
rzp_wait_healthz demo "$PORT" "$PID" 30

if [ "$#" -gt 0 ]; then
	# mise runs a root file task from the repository root, but a wrapped command
	# belongs where it was typed: `mise run //:demo -- bunx playwright test`
	# inside a //docs/user task has to see docs/user, the way calling a script
	# in that directory used to.
	cd "${MISE_ORIGINAL_CWD:-.}"
	"$@"
	exit
fi

echo "demo: http://localhost:$PORT - log in as demo / demo1234"
if [ -n "$RZP_OWNED_DATA_DIR" ]; then
	echo "demo: Ctrl-C stops it and deletes its data; REZEPTE_DATA_DIR=<dir> keeps it"
else
	echo "demo: data in $RZP_DATA_DIR, kept on exit; Ctrl-C stops it"
fi
# Ctrl-C reaches the binary too and it exits on the signal, so the non-zero
# status here is the normal way this ends, not a failure to report.
wait "$PID" || true
