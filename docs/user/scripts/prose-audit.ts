// Fails on the shapes that make a user-docs page a wall of text: a paragraph
// or list item past MAX_BLOCK_WORDS, a sentence past MAX_SENTENCE_WORDS, and a
// page whose prose runs past MAX_PAGE_WORDS. Tables, code, JSX lines, headings
// and frontmatter are not prose and are not counted. The generated API
// reference has no prose, so it is skipped. Release notes published before
// the audit existed are a record that is not rewritten; every later release
// page is audited like any other. The rules behind the numbers live in the
// memory page conventions/user-docs.
import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';

export const MAX_BLOCK_WORDS = 80;
export const MAX_SENTENCE_WORDS = 35;
export const MAX_PAGE_WORDS = 1200;

const SKIP = [/^content\/api\/reference\//, /^content\/\(releases\)\/changelog\/v1\.([0-3]\.\d+|4\.0)(-rc\d+)?\.mdx$/];

const words = (s: string) => s.split(/\s+/).filter(Boolean).length;
// Link targets and inline markup are not words a reader reads.
const plain = (s: string) => s.replace(/\[([^\]]*)\]\([^)]*\)/g, '$1').replace(/<[^>]+>/g, ' ');

export function audit(rel: string, src: string): string[] {
	const problems: string[] = [];
	const lines = src.split('\n');
	let i = 0;
	if (lines[0] === '---') {
		i = lines.indexOf('---', 1) + 1;
	}
	let fence = false;
	let block: { start: number; text: string } | undefined;
	let total = 0;
	const endBlock = () => {
		if (!block) return;
		const n = words(block.text);
		total += n;
		if (n > MAX_BLOCK_WORDS) problems.push(`${rel}:${block.start + 1}: paragraph of ${n} words (max ${MAX_BLOCK_WORDS}) - split it, or make it steps, a list or a table`);
		for (const s of block.text.split(/(?<=[.!?:])\s+(?=[A-Z"“(*`])/)) {
			const m = words(s);
			if (m > MAX_SENTENCE_WORDS) problems.push(`${rel}:${block.start + 1}: sentence of ${m} words (max ${MAX_SENTENCE_WORDS}): "${s.slice(0, 60)}…"`);
		}
		block = undefined;
	};
	for (; i < lines.length; i++) {
		const t = lines[i].trim();
		if (t.startsWith('```')) {
			endBlock();
			fence = !fence;
			continue;
		}
		if (fence) continue;
		// A tag opened without any `>` on its line (<TypeTable type={{ … }} />)
		// carries props, not prose, until the line that closes it.
		if (/^<[A-Za-z]/.test(t) && !t.includes('>')) {
			endBlock();
			while (i < lines.length && !/>$/.test(lines[i].trim())) i++;
			continue;
		}
		if (t === '' || /^(#|\||import\s|export\s)/.test(t)) {
			endBlock();
			continue;
		}
		// A line of tags only (<Callout …>, </Step>, <Screenshot … />) ends a
		// block; a line with text beside its tags (<Callout>…</Callout>) is prose.
		if (t.startsWith('<')) {
			endBlock();
			if (plain(t).trim() === '') continue;
		}
		if (/^([-*]|\d+\.)\s/.test(t)) endBlock();
		block ??= { start: i, text: '' };
		block.text += ' ' + plain(t.replace(/^([-*]|\d+\.)\s/, ''));
	}
	endBlock();
	// Fumadocs builds the table of contents at module level, where the globally
	// registered MDX components are not in scope: a heading's badge needs the
	// page's own import, and a missing one fails without an error.
	for (const c of ['Audience', 'NewIn']) {
		if (new RegExp(`^\\s*#+ .*<${c}\\b`, 'm').test(src) && !src.includes(`import { ${c} }`)) {
			problems.push(`${rel}: a heading uses <${c}> but the page does not import it - see conventions/user-docs`);
		}
	}
	if (total > MAX_PAGE_WORDS) problems.push(`${rel}: ${total} words of prose (max ${MAX_PAGE_WORDS}) - cut, or move reference detail into a table`);
	return problems;
}

function walk(dir: string): string[] {
	return readdirSync(dir, { recursive: true, encoding: 'utf8' }).filter((f) => f.endsWith('.mdx')).map((f) => path.join(dir, f));
}

if (import.meta.main) {
	const problems = walk('content')
		.filter((f) => !SKIP.some((r) => r.test(f)))
		.flatMap((f) => audit(f, readFileSync(f, 'utf8')));
	for (const p of problems) console.error(p);
	if (problems.length) {
		console.error(`\nprose-audit: ${problems.length} problem(s). The rules: docs/memory/content/conventions/user-docs.mdx`);
		process.exit(1);
	}
	console.log('prose-audit: ok');
}
