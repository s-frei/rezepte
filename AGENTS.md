# Rezepte

Self-hosted recipe manager: Go service (`service/`), SvelteKit SPA (`frontend/`), two Fumadocs sites (`docs/memory/`, `docs/user/`).
Read `docs/memory/content/index.mdx` before making changes. The memory explains the how and why; this file only lists hard rules.

## Hard rules

- Everything in the repo is English (code, comments, commits, docs). UI copy lives only in `frontend/messages/*.json`, one catalogue per language; `en.json` is the source and `de.json` is its equal, and both hold the same keys. → `docs/memory/content/conventions/svelte.mdx`
- Commits: semantic prefix, title ≤ 70 chars, body only when needed, no Co-Authored-By or tool trailers. → `docs/memory/content/conventions/commits.mdx`
- Git flow: develop on `develop`, releases merge to `main`; never push, merge to `main`, tag or open PRs without being asked. History on `develop` is never rewritten unless explicitly asked; a branch is tidied in its worktree before it lands. → `docs/memory/content/conventions/commits.mdx`
- Never commit `docs/superpowers/` (local specs and plans) or `docs/design_handoff_rezepte/` (local design mockups).
- Run tools only through mise: `mise run <task>` or `mise exec -- <cmd>`. Never call `npm`/`node` directly; use Bun. → `docs/memory/content/architecture/toolchain.mdx`
- GitHub Actions: every step carries a quoted `name`, actions are pinned to a major tag, and tools come from mise. → `docs/memory/content/conventions/github-actions.mdx`
- `mise run check` must pass before every commit. → `docs/memory/content/howtos/run-dev.mdx`
- The memory holds one page per thing that exists, never a record per event: a feature has a `features/` page, the system's shape lives in `architecture/`. Rewrite the page when the thing changes; delete it when the thing is gone. → `docs/memory/content/howtos/update-the-memory.mdx`
- Go: standard library first, `golangci-lint` clean, stdlib `testing` only, errors wrapped with `%w`. → `docs/memory/content/conventions/go.mdx`
- Svelte: Svelte 5 runes, TypeScript, Bits UI for primitives, validate every `.svelte` file with the svelte MCP `svelte-autofixer` tool before committing. → `docs/memory/content/conventions/svelte.mdx`
- UI uses only the design tokens from `frontend/src/app.css`; no raw colours, radii or fonts in components. → `docs/memory/content/architecture/design-system.mdx`
- Verify UI changes with Playwright (`mise run e2e`, or screenshots against the dev server), never with a browser extension. → `docs/memory/content/howtos/verify-ui.mdx`
- Database changes go through goose migrations and sqlc; regenerate with `mise run //service:generate` and commit the output. → `docs/memory/content/architecture/data-model.mdx`
- Docs are two independent Fumadocs apps, `docs/memory/` (agent memory, for the people developing Rezepte) and `docs/user/` (end-user docs, for the people cooking from it and hosting it), each with its own `package.json`, both English. → `docs/memory/content/architecture/docs.mdx`
- Feature work happens in a worktree created with `mise run worktree:new <name>` (branches off `develop`, assigns a port offset). Never hand-edit ports; `mise run ports` shows the map. → `docs/memory/content/howtos/work-in-a-worktree.mdx`
- The memory describes the repository, not the machine and not the day: it states what is true now, and it never records one machine's setup, paths or quirks. → `docs/memory/content/howtos/update-the-memory.mdx`
