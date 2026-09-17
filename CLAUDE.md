# Rezepte

Self-hosted recipe manager: Go service (`service/`), SvelteKit SPA (`frontend/`), Fumadocs site (`docs/`).
Read `docs/memory/index.mdx` before making changes. The memory explains the how and why; this file only lists hard rules.

## Hard rules

- Everything in the repo is English (code, comments, commits, docs). UI copy is German and lives only in `frontend/messages/*.json`. → `docs/memory/conventions/svelte.mdx`
- Commits: semantic prefix, title ≤ 70 chars, body only when needed, no Co-Authored-By or tool trailers. → `docs/memory/conventions/commits.mdx`
- Git flow: develop on `develop`, releases merge to `main`; never push, merge to `main`, tag or open PRs without being asked. → `docs/memory/conventions/commits.mdx`
- Never commit `docs/superpowers/` (local specs and plans) or `docs/design_handoff_rezepte/` (local design mockups).
- Run tools only through mise: `mise run <task>` or `mise exec -- <cmd>`. Never call `npm`/`node` directly; use Bun. → `docs/memory/decisions/0001-monorepo-with-mise.mdx`
- `mise run check` must pass before every commit. → `docs/memory/howtos/run-dev.mdx`
- Every architecture decision gets an ADR in `docs/memory/decisions/`. → `docs/memory/howtos/write-an-adr.mdx`
- Go: standard library first, `golangci-lint` clean, stdlib `testing` only, errors wrapped with `%w`. → `docs/memory/conventions/go.mdx`
- Svelte: Svelte 5 runes, TypeScript, Bits UI for primitives, validate every `.svelte` file with the svelte MCP `svelte-autofixer` tool before committing. → `docs/memory/conventions/svelte.mdx`
- UI uses only the design tokens from `frontend/src/app.css`; no raw colours, radii or fonts in components. → `docs/memory/architecture/design-system.mdx`
- Verify UI changes with Playwright (`mise run e2e`, or screenshots against the dev server), never with a browser extension. → `docs/memory/howtos/verify-ui.mdx`
- Database changes go through goose migrations and sqlc; regenerate with `mise run //service:generate` and commit the output. → `docs/memory/decisions/0003-sqlite-with-sqlc-and-goose.mdx`
- Docs content lives in `docs/memory/` and `docs/user/`, uses Fumadocs components, and is English. → `docs/memory/decisions/0005-fumadocs-as-agent-memory.mdx`
