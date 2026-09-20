// Writes the running demo binary's OpenAPI document to public/openapi.json.
// Run through `mise run //docs/user:openapi`, which starts the binary on its
// worktree's demo port.
//
// The document is behind the session
// (docs/memory/content/features/users-and-auth.mdx), so this logs in as the
// demo admin first and replays the cookie. That is also why the task needs a
// demo instance rather than any running server: it has to be one whose
// credentials we know.
import { writeFileSync } from 'node:fs';

import { DEMO_USER } from './demo-user';

const base = process.env.SCREENSHOT_BASE_URL ?? `http://localhost:${process.env.RZP_DEMO_PORT ?? 8070}`;

const login = await fetch(`${base}/api/v1/auth/login`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(DEMO_USER),
});
if (!login.ok) throw new Error(`${base}/api/v1/auth/login: ${login.status}`);
// getSetCookie keeps the cookies separate; Headers.get would join them into
// one comma-separated string that a server is not obliged to parse back.
const session = login.headers
  .getSetCookie()
  .map((c) => c.split(';', 1)[0])
  .join('; ');
if (!session) throw new Error(`${base}/api/v1/auth/login: no Set-Cookie in the response`);

const res = await fetch(`${base}/api/v1/openapi.json`, { headers: { Cookie: session } });
if (!res.ok) throw new Error(`${base}/api/v1/openapi.json: ${res.status}`);
const doc = await res.json();
writeFileSync('public/openapi.json', JSON.stringify(doc, null, 2) + '\n');
console.log(`openapi.json: ${Object.keys(doc.paths).length} paths, version ${doc.info.version}`);
