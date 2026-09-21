/**
 * Both catalogues are complete at every commit. Paraglide would quietly fall
 * back to the base locale for a missing key, which is exactly the failure
 * this repository does not want: German is not a translation of English, it
 * is the second maintained catalogue. So a difference is an error, not a
 * fallback.
 */
import { readdirSync } from 'node:fs';
import { join } from 'node:path';

const dir = join(import.meta.dir, '..', 'messages');
const files = readdirSync(dir).filter((f) => f.endsWith('.json'));

const keys = new Map<string, Set<string>>();
for (const file of files) {
	const parsed = await Bun.file(join(dir, file)).json();
	// $schema is metadata, not copy.
	keys.set(file, new Set(Object.keys(parsed).filter((k) => k !== '$schema')));
}

const union = new Set<string>();
for (const set of keys.values()) {
	for (const k of set) union.add(k);
}

// No catalogues, or nothing but empty ones, would pass every comparison below
// without comparing anything - a green gate over a missing interface.
if (files.length < 2 || union.size === 0) {
	console.error(
		`messages/ holds ${files.length} catalogue(s) with ${union.size} key(s) between them; expected at least two catalogues with keys.`
	);
	process.exit(1);
}

let failed = false;
for (const [file, set] of keys) {
	const missing = [...union].filter((k) => !set.has(k)).sort();
	if (missing.length > 0) {
		failed = true;
		console.error(`${file} is missing ${missing.length} key(s):`);
		for (const k of missing) console.error(`  ${k}`);
	}
}

if (failed) {
	console.error('\nEvery catalogue in messages/ holds the same keys. Add the missing ones.');
	process.exit(1);
}
console.log(`message parity: ${files.length} catalogues, ${union.size} keys each`);
