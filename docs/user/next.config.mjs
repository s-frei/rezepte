import { join } from 'node:path';

import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

const basePath = '/rezepte';

/** @type {import('next').NextConfig} */
const config = {
  output: 'export',
  reactStrictMode: true,
  // Every route exports as `<route>/index.html` instead of a flat
  // `<route>.html` sibling (see docs/memory/next.config.mjs for the long version).
  trailingSlash: true,
  // Don't auto-generate AGENTS.md and CLAUDE.md on dev/build; the repo already
  // has its own root CLAUDE.md.
  agentRules: false,
  // Static export has no image optimizer; <Screenshot> uses next/image via
  // ImageZoom.
  images: { unoptimized: true },
  // Published at https://s-frei.github.io/rezepte, set unconditionally so
  // the dev server matches production instead of drifting behind a CI flag.
  basePath,
  env: { NEXT_PUBLIC_BASE_PATH: basePath },
  turbopack: {
    // The repository root, not this directory. It still stops Turbopack
    // searching above the repo for a root (the "ignored bun.lock in <home dir>"
    // warning this setting exists for), and it is what lets the getting-started
    // pages `<include>` the repository's docker-compose.yaml: the include
    // plugin reads the file directly, but registers it with the bundler, which
    // rejects anything outside this root.
    root: join(import.meta.dirname, '..', '..'),
  },
};

export default withMDX(config);
