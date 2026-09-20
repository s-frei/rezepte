import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

/** @type {import('next').NextConfig} */
const config = {
  output: 'export',
  reactStrictMode: true,
  // Without this, a route with children (e.g. `/architecture`) exports as a
  // flat `architecture.html` sibling of the `architecture/` directory instead
  // of `architecture/index.html`, and leaf routes like `/roadmap` export as
  // `roadmap.html` with no `roadmap/` directory at all. trailingSlash makes
  // every route, parent or leaf, export as `<route>/index.html`.
  trailingSlash: true,
  // Don't auto-generate AGENTS.md and CLAUDE.md on dev/build; the repo
  // already has its own root CLAUDE.md.
  agentRules: false,
  turbopack: {
    // Silences "ignored bun.lock in <home dir>" — a stray lockfile above the
    // repo would otherwise make Turbopack search outside it for the root.
    root: import.meta.dirname,
  },
};

export default withMDX(config);
