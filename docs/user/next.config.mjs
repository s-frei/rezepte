import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

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
  turbopack: {
    // Silences "ignored bun.lock in <home dir>" — a stray lockfile above the
    // repo would otherwise make Turbopack search outside it for the root.
    root: import.meta.dirname,
  },
};

export default withMDX(config);
