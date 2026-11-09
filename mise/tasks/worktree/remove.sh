#!/usr/bin/env bash
#MISE description="Remove a worktree made by worktree:new, and its branch once merged"
#USAGE arg "<name>" help="Directory name under .worktrees/"
# The counterpart to worktree:new. See
# docs/memory/content/howtos/work-in-a-worktree.mdx.
set -euo pipefail

NAME="$usage_name"
# Same anchor as worktree:new: the main checkout, wherever this runs from.
ROOT="$(dirname "$(cd "$(git rev-parse --git-common-dir)" && pwd -P)")"
DIR="$ROOT/.worktrees/$NAME"

if ! git -C "$ROOT" worktree list --porcelain | grep -qxF "worktree $DIR"; then
	echo "worktree:remove: $DIR is not a worktree" >&2
	exit 1
fi
BRANCH="$(git -C "$DIR" branch --show-current)"

# No --force: if the worktree holds changes or untracked files, git refuses
# and names them, and the decision stays with whoever runs this.
git -C "$ROOT" worktree remove "$DIR"
# IntelliJ writes .idea/ back into a project it had open; clear what is left.
rm -rf "$DIR"
echo "worktree:remove: removed $DIR"

if [ -n "$BRANCH" ]; then
	if git -C "$ROOT" merge-base --is-ancestor "$BRANCH" develop; then
		git -C "$ROOT" branch -D "$BRANCH"
	else
		echo "worktree:remove: kept $BRANCH, it is not merged into develop" >&2
	fi
fi

# The counterpart to worktree:after-new, from the same mise.local.toml; it
# runs last, so it may close the very terminal this runs in.
if mise -C "$ROOT" tasks info worktree:after-remove >/dev/null 2>&1; then
	WORKTREE_DIR="$DIR" WORKTREE_NAME="$NAME" mise -C "$ROOT" run -q worktree:after-remove ||
		echo "worktree:remove: worktree:after-remove failed" >&2
fi
