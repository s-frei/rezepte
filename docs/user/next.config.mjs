import { networkInterfaces } from 'node:os';
import { join } from 'node:path';

import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

const basePath = '/rezepte';

// Next blocks dev requests to /_next/* whose Origin is not localhost, so opening
// the dev server from another device on the LAN - a phone, say - serves the HTML
// but 403s every chunk: the page renders and then never hydrates, leaving the
// sidebar and the search dead in every browser. List this machine's own
// addresses rather than pinning one; dev only, the static export ignores it.
const lanOrigins = Object.values(networkInterfaces())
  .flat()
  .filter((iface) => iface?.family === 'IPv4' && !iface.internal)
  .map((iface) => iface.address);

/** @type {import('next').NextConfig} */
const config = {
  output: 'export',
  reactStrictMode: true,
  // Every route exports as `<route>/index.html` instead of a flat
  // `<route>.html` sibling (see docs/memory/next.config.mjs for the long version).
  trailingSlash: true,
  // Don't auto-generate AGENTS.md and CLAUDE.md on dev/build; the repo already
  // has its own root AGENTS.md.
  agentRules: false,
  // Static export has no image optimizer; <Screenshot> uses next/image via
  // ImageZoom.
  images: { unoptimized: true },
  // Published at https://s-frei.github.io/rezepte, set unconditionally so
  // the dev server matches production instead of drifting behind a CI flag.
  basePath,
  allowedDevOrigins: lanOrigins,
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
