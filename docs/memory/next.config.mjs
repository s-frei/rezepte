import { networkInterfaces } from 'node:os';

import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

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
  // Without this, a route with children (e.g. `/architecture`) exports as a
  // flat `architecture.html` sibling of the `architecture/` directory instead
  // of `architecture/index.html`, and leaf routes like `/roadmap` export as
  // `roadmap.html` with no `roadmap/` directory at all. trailingSlash makes
  // every route, parent or leaf, export as `<route>/index.html`.
  trailingSlash: true,
  // Don't auto-generate AGENTS.md and CLAUDE.md on dev/build; the repo
  // already has its own root AGENTS.md.
  agentRules: false,
  allowedDevOrigins: lanOrigins,
  turbopack: {
    // Silences "ignored bun.lock in <home dir>" — a stray lockfile above the
    // repo would otherwise make Turbopack search outside it for the root.
    root: import.meta.dirname,
  },
};

export default withMDX(config);
