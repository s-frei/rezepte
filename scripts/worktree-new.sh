#!/usr/bin/env bash
# Creates a worktree for feature work and gives it its own port offset, so
# two agents can run their stacks at once. See
# docs/memory/content/howtos/work-in-a-worktree.mdx.
set -euo pipefail

NAME="${1:-}"
if [ -z "$NAME" ]; then
	echo "worktree:new: usage: mise run worktree:new <name>" >&2
	exit 1
fi

# `git rev-parse --show-toplevel` would return the *current* worktree when run
# from inside a linked one, so this task would nest a new worktree under the
# worktree it was invoked from instead of the main checkout - orphaning it if
# that outer worktree is ever removed. `--git-common-dir` always points at the
# main checkout's .git, no matter where the task runs, so anchor ROOT to its
# parent instead.
ROOT="$(dirname "$(cd "$(git rev-parse --git-common-dir)" && pwd -P)")"
DIR="$ROOT/.claude/worktrees/$NAME"
BRANCH="feat/$NAME"

if [ -e "$DIR" ]; then
	echo "worktree:new: $DIR already exists" >&2
	exit 1
fi
if git -C "$ROOT" show-ref --verify --quiet "refs/heads/$BRANCH"; then
	echo "worktree:new: branch $BRANCH already exists" >&2
	exit 1
fi

# Serialize the scan-add-write sequence below: two `worktree:new` runs started
# close together could otherwise both see the same lowest free offset and
# both take it, handing two worktrees the same ports. `mkdir` is atomic - it
# fails if the directory already exists - which makes it a usable mutex.
# `trap` removes it on any exit, success or failure.
LOCK="$ROOT/.git/worktree-new.lock"
if ! mkdir "$LOCK" 2>/dev/null; then
	echo "worktree:new: another worktree:new looks like it's running (lock: $LOCK)" >&2
	echo "worktree:new: if it crashed and left this behind, clear it with: rmdir '$LOCK'" >&2
	exit 1
fi
trap 'rmdir "$LOCK" 2>/dev/null || true' EXIT

# The live worktrees are the register: each records its offset in its own
# mise.local.toml, so `git worktree remove` frees the offset with it and
# there is no separate file to go stale.
used=""
holders=""
while IFS= read -r path; do
	f="$path/mise.local.toml"
	[ -f "$f" ] || continue
	o=$(sed -n 's/^[[:space:]]*REZEPTE_PORT_OFFSET[[:space:]]*=[[:space:]]*"\([0-9][0-9]*\)".*/\1/p' "$f")
	[ -n "$o" ] || continue
	used="$used $o"
	holders="$holders
  $o -> $path"
done < <(git -C "$ROOT" worktree list --porcelain | sed -n 's/^worktree //p')

OFFSET=""
for c in 1 2 3 4 5 6 7 8 9; do
	case " $used " in *" $c "*) continue ;; esac
	OFFSET="$c"
	break
done
if [ -z "$OFFSET" ]; then
	echo "worktree:new: all nine offsets are in use:$holders" >&2
	echo "worktree:new: remove a worktree with 'git worktree remove <path>' first" >&2
	exit 1
fi

git -C "$ROOT" worktree add "$DIR" -b "$BRANCH" develop

cat > "$DIR/mise.local.toml" <<EOF
# This worktree's port offset. Every RZP_*_PORT in the root mise.toml derives
# from it, so this worktree's services never collide with another's.
#
# Never commit this file. .gitignore excludes it at every depth, which is what
# keeps an offset from travelling back into the main checkout and silently
# shifting its ports.
[env]
REZEPTE_PORT_OFFSET = "$OFFSET"
EOF

# mise.local.toml is new here - the worktree's other config files inherit
# trust from the main checkout (mise shares trust across worktrees by path),
# but this one has no equivalent path there. `mise run` itself auto-trusts
# what it touches, but the shell's own env hook (mise activate, triggered on
# `cd`) does not and errors with "not trusted" until something does. Trust it
# up front so that first `cd` into the worktree is clean.
mise trust --quiet "$DIR" >/dev/null 2>&1 || true

echo "worktree:new: $DIR on $BRANCH"
(cd "$DIR" && mise run ports)
