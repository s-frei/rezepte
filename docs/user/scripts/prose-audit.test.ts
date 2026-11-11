import { expect, test } from 'bun:test';
import { audit, MAX_BLOCK_WORDS, MAX_SENTENCE_WORDS } from './prose-audit';

const w = (n: number) => Array(n).fill('word').join(' ');

test('passes short prose and ignores code, tables, frontmatter and multi-line tags', () => {
	const src = ['---', `title: ${w(200)}`, '---', 'A short line.', '```', w(200), '```', `| ${w(200)} |`, '<TypeTable', `  type={{ a: '${w(200)}' }}`, '/>'].join('\n');
	expect(audit('x.mdx', src)).toEqual([]);
});

test('flags a long paragraph and a long sentence', () => {
	const p = audit('x.mdx', `${w(MAX_BLOCK_WORDS + 1)}.`);
	expect(p.some((s) => s.includes('paragraph of'))).toBe(true);
	expect(p.some((s) => s.includes('sentence of'))).toBe(true);
});

test('counts list items as their own blocks', () => {
	const item = `- Word ${w(MAX_SENTENCE_WORDS - 6)}. Word ${w(MAX_SENTENCE_WORDS - 6)}.`;
	expect(audit('x.mdx', [item, item, item].join('\n'))).toEqual([]);
});

test('flags a heading badge whose component is not imported', () => {
	expect(audit('x.mdx', '## Run timers<NewIn release="v1.3.0" />')).toHaveLength(1);
	expect(audit('x.mdx', "import { NewIn } from '@/components/new-in';\n\n## Run timers<NewIn release=\"v1.3.0\" />")).toEqual([]);
});

test('counts text inside a one-line tag and does not swallow prose after an inline tag', () => {
	expect(audit('x.mdx', `<Callout>${w(MAX_BLOCK_WORDS + 1)}</Callout>`).some((s) => s.includes('paragraph of'))).toBe(true);
	const src = [`<kbd>Ctrl</kbd> starts it`, '', `${w(MAX_BLOCK_WORDS + 1)}`, '', '<Screenshot name="a" />'].join('\n');
	expect(audit('x.mdx', src).some((s) => s.includes('paragraph of'))).toBe(true);
});
