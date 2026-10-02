#!/usr/bin/env bash
#MISE description="Remove a worktree made by worktree:new, and its branch once merged"
#USAGE arg "<name>" help="Directory name under .claude/worktrees/"
#USAGE flag "--close" help="Inside Herdr, close the worktree's workspace and its panes without asking"
# The counterpart to worktree:new. See
# docs/memory/content/howtos/work-in-a-worktree.mdx.
set -euo pipefail

NAME="$usage_name"
# Same anchor as worktree:new: the main checkout, wherever this runs from.
ROOT="$(dirname "$(cd "$(git rev-parse --git-common-dir)" && pwd -P)")"
DIR="$ROOT/.claude/worktrees/$NAME"

if ! git -C "$ROOT" worktree list --porcelain | grep -qxF "worktree $DIR"; then
	echo "worktree:remove: $DIR is not a worktree" >&2
	exit 1
fi
BRANCH="$(git -C "$DIR" branch --show-current)"

# Look the Herdr workspace up before the directory is gone.
WORKSPACE=""
if [ "${HERDR_ENV:-}" = 1 ] && command -v herdr >/dev/null 2>&1; then
	WORKSPACE="$(herdr worktree list --cwd "$ROOT" 2>/dev/null |
		DIR="$DIR" bun -e 'const j = await Bun.stdin.json(); console.log(j.result.worktrees.find((w) => w.path === process.env.DIR)?.open_workspace_id ?? "")' ||
		true)"
fi

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

# Closing a workspace takes its panes with it, so it is only ever done on an
# explicit yes - --close, or y at the prompt - and last, in case this runs
# from one of those panes.
if [ -n "$WORKSPACE" ]; then
	answer=""
	if [ "${usage_close:-}" = true ]; then
		answer=y
	elif [ -t 0 ]; then
		read -r -p "worktree:remove: close Herdr workspace $WORKSPACE ($NAME) and its panes? [y/N] " answer
	fi
	if [ "$answer" = y ] || [ "$answer" = Y ]; then
		herdr workspace close "$WORKSPACE" >/dev/null
	else
		echo "worktree:remove: Herdr workspace $WORKSPACE ($NAME) stays open; close it with: herdr workspace close $WORKSPACE (or pass --close next time)"
	fi
fi
