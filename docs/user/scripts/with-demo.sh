#!/usr/bin/env bash
# Runs a command against `rezepte --demo` on its worktree's demo port with a
# throwaway data directory: builds nothing (the mise task depends on
# //service:build), starts the binary, waits for /healthz (seeding happens
# before it answers), runs the command, stops the binary and deletes the
# directory.
set -euo pipefail
# This worktree's demo port (root mise.toml [env]); 8070 when run outside mise.
PORT="${RZP_DEMO_PORT:-8070}"
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
BIN="$ROOT/service/bin/rezepte"
if [ ! -x "$BIN" ]; then
	echo "with-demo: $BIN missing, run: mise run build" >&2
	exit 1
fi
# :$PORT has to be free before we start. Our binary seeds for seconds before
# it listens, so a stranger already holding the port would answer /healthz
# first and the command below would run against their instance - silently
# fetching their OpenAPI document or screenshotting their recipes.
if (exec 3<>/dev/tcp/127.0.0.1/"$PORT") 2>/dev/null; then
	echo "with-demo: :$PORT is already in use, stop that process first" >&2
	exit 1
fi
DATA_DIR="$(mktemp -d)"
# -u clears REZEPTE_ADMIN_USER/PASSWORD from the shell's environment so a
# developer who has them exported still gets the demo/demo1234 credentials
# the screenshot specs log in with, instead of whatever they last used.
env -u REZEPTE_ADMIN_USER -u REZEPTE_ADMIN_PASSWORD \
	REZEPTE_ADDR=":$PORT" REZEPTE_DATA_DIR="$DATA_DIR" REZEPTE_LOG_LEVEL=warn "$BIN" --demo &
PID=$!
trap 'kill "$PID" 2>/dev/null || true; wait "$PID" 2>/dev/null || true; rm -rf "$DATA_DIR"' EXIT
# Seeding twelve recipes and nine placeholder images takes a moment, and the
# server only answers once it is done - hence 30s rather than the 5s the e2e
# task waits for an empty instance. `kill -0` catches our process dying during
# that window (a failed migration, a crash) instead of waiting out the loop.
for _ in $(seq 1 300); do
	if ! kill -0 "$PID" 2>/dev/null; then
		echo "with-demo: rezepte exited before answering on :$PORT" >&2
		exit 1
	fi
	curl -sf "localhost:$PORT/healthz" >/dev/null && break
	sleep 0.1
done
kill -0 "$PID" 2>/dev/null || { echo "with-demo: rezepte exited before answering on :$PORT" >&2; exit 1; }
curl -sf "localhost:$PORT/healthz" >/dev/null || { echo "with-demo: rezepte did not start on :$PORT" >&2; exit 1; }
"$@"
