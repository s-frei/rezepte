// Writes the running demo binary's OpenAPI document to public/openapi.json.
// Run through `mise run //docs/user:openapi`, which starts the binary on :8070.
import { writeFileSync } from 'node:fs';

const base = process.env.SCREENSHOT_BASE_URL ?? 'http://localhost:8070';
const res = await fetch(`${base}/api/v1/openapi.json`);
if (!res.ok) throw new Error(`${base}/api/v1/openapi.json: ${res.status}`);
const doc = await res.json();
writeFileSync('public/openapi.json', JSON.stringify(doc, null, 2) + '\n');
console.log(`openapi.json: ${Object.keys(doc.paths).length} paths, version ${doc.info.version}`);
