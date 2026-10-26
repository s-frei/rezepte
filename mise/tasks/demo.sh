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
trap 'rzp_stop "${PID:-}"; [ -n "${STUB_PID:-}" ] && kill "$STUB_PID" 2>/dev/null; rm -f "${jar:-}" "${stub_out:-}"' EXIT
# Sign-in through a provider, by RZP_DEMO_OIDC:
# - dex (the default): Dex, when it answers (mise run oidc:up);
# - stub: mise/lib/stub-issuer.ts, a provider that only answers discovery, so
#   the screenshots show the same configured state whether Dex runs or not;
# - off: none.
# Either provider is saved through the settings API after start, like mail
# below, never REZEPTE_OIDC_*, which would lock the Sign-in card; jonas is
# linked to his identity there by --demo-oidc-issuer.
OIDC_ISSUER=""
case "${RZP_DEMO_OIDC:-dex}" in
dex) rzp_dex_up && OIDC_ISSUER="$(rzp_dex_issuer)" ;;
stub)
	stub_out="$(mktemp)"
	bun mise/lib/stub-issuer.ts >"$stub_out" &
	STUB_PID=$!
	for _ in $(seq 1 50); do
		[ -s "$stub_out" ] && break
		sleep 0.1
	done
	OIDC_ISSUER="$(head -n 1 "$stub_out")"
	[ -n "$OIDC_ISSUER" ] || {
		echo "demo: the stub provider did not start" >&2
		exit 1
	}
	;;
off) ;;
*)
	echo "demo: RZP_DEMO_OIDC is dex, stub or off, not $RZP_DEMO_OIDC" >&2
	exit 1
	;;
esac
# The public URL is set even without Mailpit or a provider: the mail and
# sign-in cards stay usable, so a server or provider can be entered by hand.
# With Mailpit up (mise run mail:up) the demo sends mail into it. Configured
# through the API after start, into the database - never REZEPTE_SMTP_*,
# which would lock the mail card the demo is there to click through.
MAIL_ENV=()
[ -z "${REZEPTE_PUBLIC_URL:-}" ] && MAIL_ENV=(REZEPTE_PUBLIC_URL="http://localhost:$PORT")
if [ "${RZP_DEMO_MAIL:-on}" != off ] && rzp_mailpit_up; then
	DEMO_MAIL=1
fi
# "${MAIL_ENV[@]+"${MAIL_ENV[@]}"}" below, not a bare "${MAIL_ENV[@]}": on
# bash < 4.4 (macOS ships 3.2 as /bin/bash) an empty array expands to an
# unbound variable under `set -u`. The `+` form expands to nothing at all
# when the array is empty instead of touching it.
# -u clears REZEPTE_ADMIN_USER and REZEPTE_ADMIN_PASSWORD from the environment
# so a developer who has them exported still gets the demo/demo1234 credentials
# the screenshot specs and the OpenAPI fetch log in with, instead of whatever
# they last used. It clears REZEPTE_OIDC_* too: an exported provider would
# lock the Sign-in card and override the one saved below.
#
# RZP_DEMO_LOCALE, when set, beats REZEPTE_LOCALE: a caller that needs one
# language (the screenshot tasks need English) cannot set REZEPTE_LOCALE
# itself, because this task's own config env - a developer's mise.local.toml
# included - is applied again on the way in and would override it.
#
# RZP_DEMO_NOW, an RFC 3339 instant, dates the samples, their comments and the
# demo tokens from
# it instead of from now: the screenshot tasks set it to the browser's frozen
# clock, so "Today" and "Yesterday" fall on the same comments every run.
env -u REZEPTE_ADMIN_USER -u REZEPTE_ADMIN_PASSWORD \
	-u REZEPTE_OIDC_ISSUER -u REZEPTE_OIDC_CLIENT_ID -u REZEPTE_OIDC_CLIENT_SECRET -u REZEPTE_OIDC_NAME \
	REZEPTE_ADDR=":$PORT" REZEPTE_DATA_DIR="$RZP_DATA_DIR" REZEPTE_LOG_LEVEL=warn \
	REZEPTE_LOCALE="${RZP_DEMO_LOCALE:-${REZEPTE_LOCALE:-en}}" \
	"${MAIL_ENV[@]+"${MAIL_ENV[@]}"}" \
	service/bin/rezepte --demo ${RZP_DEMO_NOW:+--demo-now="$RZP_DEMO_NOW"} \
	${OIDC_ISSUER:+--demo-oidc-issuer="$OIDC_ISSUER"} &
PID=$!
# Seeding twelve recipes and nine placeholder images takes a moment, and the
# server answers only once it is done - hence 30s rather than the 5s an empty
# instance needs.
rzp_wait_healthz demo "$PORT" "$PID" 30

# Best-effort: a failed step leaves a demo without mail or sign-in through a
# provider, never no demo. The cookie jar goes with the EXIT trap.
if [ "${DEMO_MAIL:-}" = 1 ] || [ -n "$OIDC_ISSUER" ]; then
	jar="$(mktemp)"
	rzp_login "$PORT" "$jar" demo demo1234 || echo "demo: could not sign in to configure mail and sign-in" >&2
fi
demo_mail() {
	local base="http://localhost:$PORT" current
	current="$(curl -fsS -b "$jar" "$base/api/v1/settings/mail")" || return 1
	# Already set up (a kept data dir): leave it as it is.
	case "$current" in *'"source":"none"'*) ;; *) return 0 ;; esac
	curl -fsS -b "$jar" -X PUT -H "Origin: $base" -H 'Content-Type: application/json' \
		-d "{\"host\":\"localhost\",\"port\":$RZP_MAILPIT_SMTP_PORT,\"security\":\"none\",\"from\":\"rezepte@example.com\",\"fromName\":\"Rezepte\"}" \
		"$base/api/v1/settings/mail" >/dev/null
}
if [ "${DEMO_MAIL:-}" = 1 ]; then
	if demo_mail; then
		echo "demo: mail goes to Mailpit - inbox http://localhost:$RZP_MAILPIT_UI_PORT"
	else
		echo "demo: mail not configured" >&2
	fi
fi
if [ "${RZP_DEMO_OIDC:-dex}" = stub ] && [ -n "$OIDC_ISSUER" ]; then
	# An empty name: the button reads "Continue with single sign-on".
	rzp_oidc_configure "$PORT" "$jar" "$OIDC_ISSUER" screenshots "" "" ||
		echo "demo: sign-in provider not configured" >&2
elif [ -n "$OIDC_ISSUER" ]; then
	if rzp_oidc_configure "$PORT" "$jar"; then
		echo "demo: signing in through Dex is on - sign in as demo@, mila@ or jonas@example.com, password <name>1234"
	else
		echo "demo: sign-in provider not configured" >&2
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
