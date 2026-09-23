import { createOpenAPI } from 'fumadocs-openapi/server';

// Server-side only: it reads the document from disk at build time, both for
// `generateFiles` (scripts/generate-api.ts) and for the preloaded props the
// page renderer hands to <OpenAPIPage>.
//
// public/openapi.json is the committed document `mise run //docs/user:openapi`
// fetches from a demo instance; nothing here talks to a running server.
//
// No `proxyUrl`: a proxy is a route handler, and this app is a static export
// (next.config.mjs, `output: 'export'`). The playground that would need one is
// disabled in components/api-page.tsx.
export const openapi = createOpenAPI({ input: ['./public/openapi.json'] });
