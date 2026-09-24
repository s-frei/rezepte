// The changelog page of a release: its draft, the checks it has to pass
// before a tag, and the GitHub release body built from its frontmatter.
// One page per release lives in docs/user/content/changelog/. See
// docs/memory/content/architecture/releases.mdx.
import type { Classified } from './version';

// A copy of SITE_URL in docs/user/lib/shared.ts, not an import: the docs app
// and the release tooling stay independent. Change both together.
export const DOCS_URL = 'https://s-frei.github.io/rezepte/';
export const CHANGELOG_DIR = 'docs/user/content/changelog';
export const DRAFT_MARKER = '{/* DRAFT */}';
export const FILL_IN = 'FILL IN';

export type Frontmatter = { title?: string; description?: string; date?: string; upgrade?: string };

const FRONTMATTER = /^---\n([\s\S]*?)\n---(?:\n|$)/;

export function parseFrontmatter(src: string): { data: Frontmatter } | { error: string } {
	const m = FRONTMATTER.exec(src);
	if (!m) return { error: 'no frontmatter block' };
	let raw: unknown;
	try {
		raw = Bun.YAML.parse(m[1]);
	} catch (e) {
		return { error: (e as Error).message };
	}
	if (raw === null || typeof raw !== 'object') return { error: 'frontmatter is not a mapping' };
	const r = raw as Record<string, unknown>;
	const data: Frontmatter = {};
	for (const key of ['title', 'description', 'upgrade'] as const) {
		if (r[key] !== undefined && r[key] !== null) data[key] = String(r[key]);
	}
	// YAML reads an unquoted 2026-10-02 as a timestamp; keep the day only.
	if (r.date instanceof Date) data.date = r.date.toISOString().slice(0, 10);
	else if (r.date !== undefined && r.date !== null) data.date = String(r.date);
	return { data };
}

// Subjects go into an MDX comment; a literal */ would close it early.
const defuse = (s: string) => s.replaceAll('*/', '* /');

export function draftPage(version: string, date: string, lastTag: string | null, c: Classified): string {
	const list = (title: string, subjects: string[]) =>
		subjects.length ? [`${title}:`, ...subjects.map((s) => `- ${defuse(s)}`), ''] : [];
	// Quoted, so whatever replaces FILL IN stays one string: unquoted YAML cuts a
	// value at ` #` and fails on a leading backtick or an inner `: `.
	return [
		'---',
		`title: "${version} — ${FILL_IN}"`,
		`description: "${FILL_IN}"`,
		`date: ${date}`,
		...(c.breaking.length ? [`upgrade: "${FILL_IN}"`] : []),
		'---',
		'',
		DRAFT_MARKER,
		'',
		'{/*',
		`${lastTag ? `Commits since ${lastTag}` : 'All commits so far'} - the raw material. Rewrite them for`,
		'the people who run and use Rezepte, then delete this comment and the DRAFT line.',
		'',
		...list('Breaking', c.breaking),
		...list('Features', c.features),
		...list('Other', c.other),
		'*/}',
		'',
		`## ${FILL_IN}`,
		'',
		FILL_IN,
		'',
		'## Also in this release',
		'',
		`- **New:** ${FILL_IN}`,
		`- **Fixed:** ${FILL_IN}`,
		''
	].join('\n');
}

export function insertIntoMeta(metaJson: string, version: string): string {
	const meta = JSON.parse(metaJson) as { pages?: string[] };
	const pages = meta.pages ?? ['index'];
	if (pages.includes(version)) throw new Error(`${version} is already in ${CHANGELOG_DIR}/meta.json`);
	const at = pages.indexOf('index') + 1;
	meta.pages = [...pages.slice(0, at), version, ...pages.slice(at)];
	return `${JSON.stringify(meta, null, '\t')}\n`;
}

export function checkPage(src: string, hasBreaking: boolean): string[] {
	const problems: string[] = [];
	if (src.includes(DRAFT_MARKER)) problems.push('the DRAFT marker is still there');
	if (src.includes(FILL_IN)) problems.push(`"${FILL_IN}" is still there`);
	if (problems.length) return problems;
	const fm = parseFrontmatter(src);
	if ('error' in fm) return [`frontmatter does not parse: ${fm.error}`];
	for (const key of ['title', 'description', 'date'] as const) {
		if (!fm.data[key]) problems.push(`frontmatter has no ${key}`);
	}
	if (hasBreaking && !fm.data.upgrade) problems.push('the release has breaking commits but no upgrade notice');
	return problems;
}

export const pageUrl = (version: string) => `${DOCS_URL}changelog/${version}/`;
export const markdownUrl = (version: string) => `${DOCS_URL}llms.mdx/changelog/${version}/content.md`;

export function releaseBody(version: string, fm: Frontmatter): string {
	return [
		...(fm.upgrade ? [`**Before you upgrade:** ${fm.upgrade}`, ''] : []),
		fm.description ?? '',
		'',
		`📖 Release notes: ${pageUrl(version)}`,
		`🤖 For LLMs: ${markdownUrl(version)}`,
		''
	].join('\n');
}
