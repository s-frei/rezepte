#!/usr/bin/env bash
#MISE description="Start rezepte --demo on this worktree's demo port; with arguments, run them against it instead"
#MISE depends=["//:build"]
#USAGE arg "[command]..." var=#true double_dash="required" help="A command to run against the running demo instance, after --; it is stopped afterwards"
#
# `mise run demo` starts the binary with demo data and keeps it up until Ctrl-C
# - the fastest way to a realistic instance to click through or point a probe
# at. `mise run demo -- <cmd>` runs one command against it and stops it
# afterwards, which is how the screenshot and OpenAPI tasks get an instance
# they own. See docs/memory/content/features/demo-mode.mdx.
set -euo pipefail
. mise/lib/instance.sh

: "${RZP_DEMO_PORT:?run this through mise: mise run demo}"
# RZP_DEMO_INSTANCE_PORT, when set, beats RZP_DEMO_PORT, for the reason
# RZP_DEMO_LOCALE beats REZEPTE_LOCALE below: the screenshot and OpenAPI
# tasks start their own instance on their own port, so it runs beside a demo
# someone is clicking through, and this task's config env would put
# RZP_DEMO_PORT back on the way in.
PORT="${RZP_DEMO_INSTANCE_PORT:-$RZP_DEMO_PORT}"

rzp_require_binary demo
rzp_require_free_port demo "$PORT"
rzp_make_data_dir
# With the test provider up (mise run oidc:up) the demo signs in through it
# too. A caller that configures OIDC itself - the screenshot task, which needs
# the same picture whether Dex runs or not - is left alone.
if [ -n "${REZEPTE_OIDC_ISSUER:-}" ]; then
	OIDC_ENV=()
else
	rzp_oidc_env "$PORT"
	[ "${#OIDC_ENV[@]}" -gt 0 ] && echo "demo: signing in through Dex is on - sign in as demo@, mila@ or jonas@example.com, password <name>1234"
fi
# With Mailpit up (mise run mail:up) the demo sends mail into it. Configured
# through the API after start, into the database - never REZEPTE_SMTP_*,
# which would lock the mail card the demo is there to click through.
MAIL_ENV=()
if [ "${RZP_DEMO_MAIL:-on}" != off ] && rzp_mailpit_up; then
	DEMO_MAIL=1
	[ -z "${REZEPTE_PUBLIC_URL:-}" ] && MAIL_ENV=(REZEPTE_PUBLIC_URL="http://localhost:$PORT")
fi
# "${OIDC_ENV[@]+"${OIDC_ENV[@]}"}" below, not a bare "${OIDC_ENV[@]}": on
# bash < 4.4 (macOS ships 3.2 as /bin/bash) an empty array expands to an
# unbound variable under `set -u`. The `+` form expands to nothing at all
# when the array is empty instead of touching it.
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
	"${OIDC_ENV[@]+"${OIDC_ENV[@]}"}" \
	"${MAIL_ENV[@]+"${MAIL_ENV[@]}"}" \
	service/bin/rezepte --demo &
PID=$!
trap 'rzp_stop "$PID"; rm -f "${jar:-}"' EXIT
# Seeding twelve recipes and nine placeholder images takes a moment, and the
# server answers only once it is done - hence 30s rather than the 5s an empty
# instance needs.
rzp_wait_healthz demo "$PORT" "$PID" 30

# Best-effort: a failed step leaves a demo without mail, never no demo. The
# cookie jar goes with the EXIT trap.
demo_mail() {
	local base="http://localhost:$PORT" current
	curl -fsS -c "$jar" -H "Origin: $base" -H 'Content-Type: application/json' \
		-d '{"username":"demo","password":"demo1234"}' "$base/api/v1/auth/login" >/dev/null || return 1
	current="$(curl -fsS -b "$jar" "$base/api/v1/settings/mail")" || return 1
	# Already set up (a kept data dir): leave it as it is.
	case "$current" in *'"source":"none"'*) ;; *) return 0 ;; esac
	curl -fsS -b "$jar" -X PUT -H "Origin: $base" -H 'Content-Type: application/json' \
		-d "{\"host\":\"localhost\",\"port\":$RZP_MAILPIT_SMTP_PORT,\"security\":\"none\",\"from\":\"rezepte@example.com\",\"fromName\":\"Rezepte\"}" \
		"$base/api/v1/settings/mail" >/dev/null
}
if [ "${DEMO_MAIL:-}" = 1 ]; then
	jar="$(mktemp)"
	if demo_mail; then
		echo "demo: mail goes to Mailpit - inbox http://localhost:$RZP_MAILPIT_UI_PORT"
	else
		echo "demo: mail not configured" >&2
	fi
fi

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
