/**
 * Both catalogs are complete at every commit. Paraglide would quietly fall
 * back to the base locale for a missing key, which is exactly the failure
 * this repository does not want: German is not a translation of English, it
 * is the second maintained catalog. So a difference is an error, not a
 * fallback.
 *
 * Nor may a catalog name a key twice. JSON parsers keep the last value
 * without a word, so an earlier copy reads as the live copy while the later
 * one ships.
 */
import { readdirSync } from 'node:fs';
import { join } from 'node:path';

const dir = join(import.meta.dir, '..', 'messages');
const files = readdirSync(dir).filter((f) => f.endsWith('.json'));

/** The keys of the top-level object in `text`, in order, repeats included. */
function topLevelKeys(text: string): string[] {
	const found: string[] = [];
	let depth = 0;
	for (let i = 0; i < text.length; i++) {
		const c = text[i];
		if (c === '{' || c === '[') depth++;
		else if (c === '}' || c === ']') depth--;
		else if (c === '"') {
			let end = i + 1;
			while (text[end] !== '"') end += text[end] === '\\' ? 2 : 1;
			let next = end + 1;
			while (/\s/.test(text[next] ?? '')) next++;
			if (depth === 1 && text[next] === ':') found.push(JSON.parse(text.slice(i, end + 1)));
			i = end;
		}
	}
	return found;
}

let failed = false;
const keys = new Map<string, Set<string>>();
for (const file of files) {
	const text = await Bun.file(join(dir, file)).text();
	const parsed = JSON.parse(text);
	// $schema is metadata, not copy.
	keys.set(file, new Set(Object.keys(parsed).filter((k) => k !== '$schema')));

	const seen = new Set<string>();
	const repeated = new Set<string>();
	for (const k of topLevelKeys(text)) {
		if (seen.has(k)) repeated.add(k);
		seen.add(k);
	}
	if (repeated.size > 0) {
		failed = true;
		console.error(`${file} names ${repeated.size} key(s) more than once:`);
		for (const k of [...repeated].sort()) console.error(`  ${k}`);
	}
}

const union = new Set<string>();
for (const set of keys.values()) {
	for (const k of set) union.add(k);
}

// No catalogs, or nothing but empty ones, would pass every comparison below
// without comparing anything - a green gate over a missing interface.
if (files.length < 2 || union.size === 0) {
	console.error(
		`messages/ holds ${files.length} catalog(s) with ${union.size} key(s) between them; expected at least two catalogs with keys.`
	);
	process.exit(1);
}

for (const [file, set] of keys) {
	const missing = [...union].filter((k) => !set.has(k)).sort();
	if (missing.length > 0) {
		failed = true;
		console.error(`${file} is missing ${missing.length} key(s):`);
		for (const k of missing) console.error(`  ${k}`);
	}
}

if (failed) {
	console.error('\nEvery catalog in messages/ holds the same keys, each once.');
	process.exit(1);
}
console.log(`message parity: ${files.length} catalogs, ${union.size} keys each`);
