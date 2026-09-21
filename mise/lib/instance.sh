# Sourced by mise/tasks/demo.sh and mise/tasks/e2e.sh: the three things both of
# them have to get right when they start ./service/bin/rezepte themselves - the
# port guard, the data directory and the /healthz wait.
#
# It lives in mise/lib/ rather than mise/tasks/ because a file in there would
# become a task of its own, and this one is a library: it defines functions and
# runs nothing. It is sourced with a path relative to the repository root, which
# mise makes the working directory of every root file task.
#
# What stays with the callers is what actually differs between them: which
# REZEPTE_* variables they set, whether they pass --demo, and how long seeding
# makes them wait. Both copies of this logic drifted while they were two inline
# blocks - different port guards, different `env -u` lists, one of them missing
# the process-died check - which is why it is one function per concern now.

# Set by rzp_make_data_dir and read by rzp_stop.
RZP_DATA_DIR=""
RZP_OWNED_DATA_DIR=""

# rzp_require_free_port <label> <port>
#
# The port has to be free before we start. Our binary seeds and migrates for
# seconds before it listens, so a stranger already holding the port would
# answer /healthz first and everything after would silently run against their
# instance - screenshotting their recipes, fetching their OpenAPI document, or
# failing every test that logs in while the ones that do not log in pass.
# Callers may set RZP_PORT_HINT to add their own way out of it.
rzp_require_free_port() {
	local label="$1" port="$2"
	if (exec 3<>/dev/tcp/127.0.0.1/"$port") 2>/dev/null; then
		echo "$label: :$port is already in use${RZP_PORT_HINT:+ - ${RZP_PORT_HINT}}" >&2
		echo "$label: find it with: lsof -nP -iTCP:$port -sTCP:LISTEN" >&2
		exit 1
	fi
}

# rzp_make_data_dir
#
# Sets RZP_DATA_DIR. A REZEPTE_DATA_DIR the caller exported wins and is left
# alone, so `REZEPTE_DATA_DIR=.demo-data mise run demo` keeps its recipes
# across restarts (demo mode is idempotent on a non-empty instance). Without
# one, this makes a throwaway directory and records that rzp_stop owns it -
# nothing removes a directory it did not create.
rzp_make_data_dir() {
	if [ -n "${REZEPTE_DATA_DIR:-}" ]; then
		RZP_DATA_DIR="$REZEPTE_DATA_DIR"
		RZP_OWNED_DATA_DIR=""
		return
	fi
	RZP_DATA_DIR="$(mktemp -d)"
	RZP_OWNED_DATA_DIR="$RZP_DATA_DIR"
}

# rzp_require_binary <label>
rzp_require_binary() {
	if [ ! -x service/bin/rezepte ]; then
		echo "$1: service/bin/rezepte missing, run: mise run build" >&2
		exit 1
	fi
}

# rzp_wait_healthz <label> <port> <pid> <seconds>
#
# `kill -0` catches the process dying inside the window - a failed migration, a
# crash, a port taken between the guard above and the bind - instead of waiting
# out the whole loop and then blaming the timeout.
rzp_wait_healthz() {
	local label="$1" port="$2" pid="$3" seconds="$4"
	local i
	for i in $(seq 1 $((seconds * 10))); do
		if ! kill -0 "$pid" 2>/dev/null; then
			echo "$label: rezepte exited before answering on :$port" >&2
			exit 1
		fi
		curl -sf "localhost:$port/healthz" >/dev/null && return
		sleep 0.1
	done
	echo "$label: rezepte did not answer on :$port within ${seconds}s" >&2
	exit 1
}

# rzp_stop <pid> - for `trap ... EXIT`.
rzp_stop() {
	kill "$1" 2>/dev/null || true
	wait "$1" 2>/dev/null || true
	[ -n "$RZP_OWNED_DATA_DIR" ] && rm -rf "$RZP_OWNED_DATA_DIR"
	return 0
}
