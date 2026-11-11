// Fails the build when the agent memory claims something the repository does
// not support or breaks the form its writing rules set. It checks the claims
// that rot silently - a task, a path, a link or an anchor that was renamed,
// deleted or never committed - the phrasings that must never be written, the
// navigation every page has to appear in, and the length limits that keep a
// page greppable.
//
// It cannot tell whether a sentence is true, only whether the things it names
// still exist and whether the page keeps its form. Prose drift is the writing
// rules' job (content/howtos/update-the-memory.mdx), not this script's.
//
// Ports are deliberately NOT checked. Every port in the memory is either a
// default named beside its variable or a `mise run ports` pointer, and a check
// would need more exemptions than it has findings.
//
// Everything specific to this repository is in the config block below; another
// project that copies the memory's rules copies this file and rewrites only
// that block.
import { readdirSync, readFileSync, existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execSync } from 'node:child_process';

// Resolved from this file rather than the working directory, so the script
// behaves the same however it is invoked. `next build` type-checks it with the
// site's tsconfig, which has no Bun globals - hence fileURLToPath, not
// import.meta.dir.
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..');

// ---- config: the only part another project rewrites --------------------------

/** The Fumadocs content folder of the memory, relative to the repository root. */
const CONTENT = 'docs/memory/content';
/** Files whose claims are checked: the memory plus the agents' entry file. */
const SOURCES = [CONTENT, 'AGENTS.md'];
/** The writing rules quote what they forbid, so the phrasing checks skip them. */
const RULES_PAGE = `${CONTENT}/howtos/update-the-memory.mdx`;
/** The how-to index: one <Card> per how-to, carrying the page's description. */
const HOWTO_INDEX = `${CONTENT}/howtos/index.mdx`;
/** Pages under this folder carry FEATURE_SECTIONS in order. */
const FEATURE_DIR = `${CONTENT}/features/`;
/** The sections every feature page carries, in the order they must appear. */
const FEATURE_SECTIONS = ['## How it works', '## Why it is this way', '## Rejected', '## Limits'];
/**
 * Backticked repository paths that are checked against git: the package
 * directories, `mise/`, `assets/` and `.github/`, plus root files named by name.
 * `scripts/` stays although no such directory exists: a page that still points
 * at one has to fail rather than read as true.
 */
const REPO_PATH = /`((?:service|frontend|docs|scripts|mise|assets|\.github)\/[A-Za-z0-9_./*{}-]+|mise\.toml|CLAUDE\.md|AGENTS\.md)`/g;
/**
 * Gitignored artifacts named on purpose (local agent scratch space, static
 * exports, build output, data dirs); they are absent from a clean checkout.
 */
const IGNORED_PATHS =
	/^(docs\/superpowers|docs\/design_handoff_rezepte|docs\/memory\/out|docs\/user\/out|frontend\/test-results|service\/bin|data)(\/|$)/;
/** Past this a paragraph or list item is a wall nobody reads to the end. */
const MAX_PARAGRAPH_WORDS = 300;
/** Past this a `##` section needs `###` subheadings to be navigable. */
const MAX_SECTION_WORDS = 1000;

// ---- end of config -------------------------------------------------------------

/** Phrasings that tie the memory to one person's machine rather than to the repository. */
const MACHINE_INTERNALS = [
	/\bdevelopment (machine|Mac|laptop)\b/i,
	/\bon (this|my) (machine|Mac|laptop)\b/i,
	/\/Users\/[a-z]/i,
	/\bDocker Desktop\b/,
	/\bscreen is locked\b|\blocked screen\b/i
];

/**
 * Phrasings that describe an event rather than a constraint. A sentence that
 * only makes sense to a reader who knows what the repository looked like
 * before has a shelf life, and git already records the change.
 *
 * "no longer" is deliberately absent - it reads as plain negation far more
 * often than as history ("a trimmed fraction no longer sorts the same"), and a
 * rule with more false positives than findings gets ignored.
 */
const EVENT_FRAMING = [
	/\boriginally\b/i,
	/\bused to\b/i,
	/\bat the time\b/i,
	/\buntil then\b/i,
	/\bfor now\b/i,
	/\bwas tried\b/i,
	/\bgrew up\b/i,
	/\bphase [0-9]/i,
	/\b20[0-9]{2}-[01][0-9]-[0-9]{2}\b/,
	/\b(since|until|as of) (early |mid-|late )?20[0-9]{2}\b/i,
	/\bnot (built|implemented|wired) yet\b/i
];

/** Paths that are not expected to resolve, and why. */
function pathIsExempt(p: string): boolean {
	if (p.includes('*')) return true; // a glob, not a file
	if (p.includes('NNNN')) return true; // a naming template
	if (/\/[a-z]+\.[A-Z]/.test(p)) return true; // a Go symbol such as demo.Seed
	return IGNORED_PATHS.test(p);
}

/** A placeholder standing in for a real name, not a claim that one exists. */
function isPlaceholder(s: string): boolean {
	return /[…<>]/.test(s) || s.includes('NNNN');
}

function walk(dir: string): string[] {
	return readdirSync(dir, { recursive: true, encoding: 'utf8' }).filter((f) => f.endsWith('.mdx')).map((f) => path.join(dir, f));
}

/** The anchor Fumadocs gives a heading: github-slugger's rules. */
function slug(heading: string): string {
	return heading
		.toLowerCase()
		.replace(/[^\p{L}\p{N}\s_-]/gu, '')
		.trim()
		.replace(/\s/g, '-');
}

/** Lines outside fenced code blocks and frontmatter, with their 0-based index. */
function proseLines(body: string): [string, number][] {
	const out: [string, number][] = [];
	let fenced = false;
	let front = body.startsWith('---\n');
	body.split('\n').forEach((line, i) => {
		if (front) {
			if (i > 0 && line === '---') front = false;
			return;
		}
		if (/^\s*```/.test(line)) {
			fenced = !fenced;
			return;
		}
		if (!fenced) out.push([line, i]);
	});
	return out;
}

/**
 * Every page in the memory, addressed the way a link addresses it
 * (content/features/recipes.mdx -> /features/recipes, content/index.mdx -> /,
 * content/howtos/index.mdx -> /howtos), with the anchors its headings get. A
 * link or anchor that resolves to nothing is a dead end for the agent that
 * follows it, and renaming a page or a heading is exactly when one appears.
 */
function memoryRoutes(): Map<string, Set<string>> {
	const base = path.join(ROOT, CONTENT);
	const routes = new Map<string, Set<string>>();
	for (const file of walk(base)) {
		const rel = path.relative(base, file).replace(/\.mdx$/, '');
		const route = rel === 'index' ? '/' : '/' + rel.replace(/\/index$/, '');
		const anchors = new Set<string>();
		const seen = new Map<string, number>();
		for (const [line] of proseLines(readFileSync(file, 'utf8'))) {
			const h = line.match(/^\s*#{1,6}\s+(.*?)\s*$/);
			if (!h) continue;
			const base = slug(h[1]);
			const n = seen.get(base) ?? 0;
			seen.set(base, n + 1);
			anchors.add(n === 0 ? base : `${base}-${n}`);
		}
		routes.set(route, anchors);
	}
	return routes;
}

/** The `description` in a page's frontmatter. */
function description(body: string): string | undefined {
	return body.match(/^---\n[\s\S]*?^description:\s*(.*?)\s*$/m)?.[1].replace(/^(["'])(.*)\1$/, '$2');
}

const files = SOURCES.flatMap((s) => {
	const abs = path.join(ROOT, s);
	if (!existsSync(abs)) throw new Error(`memory-audit: ${s} is missing`);
	return s.endsWith('.md') ? [abs] : walk(abs);
});

// `mise tasks --all` is the only authority on what a task name means; a list
// repeated here would be the second literal this whole convention exists to
// prevent.
const tasks = new Set(
	execSync('mise tasks --all', { cwd: ROOT, encoding: 'utf8' })
		.split('\n')
		.map((l) => l.trim().split(/\s+/)[0])
		.filter(Boolean)
		.map((t) => t.replace(/^\/\/:/, ''))
);

const routes = memoryRoutes();

// A path the memory names has to reach the next checkout, not only this one:
// a file that exists here but is gitignored or local-only would pass an
// existsSync and still be a dead end for everyone else. Untracked files count,
// so a page and the file it names can be written before the first `git add`.
const inGit = new Set<string>();
for (const f of execSync('git ls-files --cached --others --exclude-standard', { cwd: ROOT, encoding: 'utf8' }).split('\n')) {
	const parts = f.split('/');
	for (let i = 1; i <= parts.length; i++) inGit.add(parts.slice(0, i).join('/'));
}
const isInGit = (p: string) => p.includes('*') || p.includes('{') ? existsSync(path.join(ROOT, p)) : inGit.has(p.replace(/\/$/, ''));

const problems: string[] = [];

for (const file of files) {
	const rel = path.relative(ROOT, file);
	const body = readFileSync(file, 'utf8');
	const self = rel.startsWith(CONTENT)
		? '/' + path.relative(path.join(ROOT, CONTENT), file).replace(/\.mdx$/, '').replace(/(^|\/)index$/, '')
		: '';
	body
		.split('\n')
		.forEach((line, i) => {
			const at = `${rel}:${i + 1}`;

			// The writing rules state what they forbid by quoting it.
			const statesTheRule = rel === RULES_PAGE;

			for (const rx of MACHINE_INTERNALS) {
				if (rx.test(line) && !statesTheRule) {
					problems.push(`${at}: names one machine's setup - ${rx}`);
				}
			}

			if (!statesTheRule) {
				for (const rx of EVENT_FRAMING) {
					if (rx.test(line)) {
						problems.push(`${at}: describes an event, not a constraint - ${rx}`);
					}
				}
			}

			for (const m of line.matchAll(/`mise run ([^`]+)`/g)) {
				// `//:name` is the explicit spelling of a root task, listed as plain `name`.
				const task = m[1].split(/\s/)[0].replace(/^\/\/:/, '');
				if (isPlaceholder(task)) continue;
				if (!tasks.has(task) && !tasks.has(`//${task}`)) {
					problems.push(`${at}: no such mise task: ${task}`);
				}
			}

			for (const m of line.matchAll(REPO_PATH			)) {
				const p = m[1];
				if (!pathIsExempt(p) && !isInGit(p)) {
					problems.push(`${at}: path is not in git: ${p}`);
				}
			}

			// The memory's how-to index addresses pages with JSX (`<Card
			// href="...">`) rather than markdown links, so both forms are checked
			// against the same route set. A bare `#anchor` points into the page
			// itself.
			for (const m of line.matchAll(/\]\(([/#][A-Za-z0-9/._#?-]*)\)|href="([/#][A-Za-z0-9/._#?-]*)"/g)) {
				const href = (m[1] ?? m[2]).replace(/\?.*$/, '');
				const [page, anchor] = href.split('#');
				const target = (page || self).replace(/\/$/, '') || '/';
				const anchors = routes.get(target);
				if (!anchors) problems.push(`${at}: link goes nowhere: ${target}`);
				else if (anchor && !anchors.has(anchor)) problems.push(`${at}: no heading for #${anchor} on ${target}`);
			}
		});

	if (rel.startsWith(FEATURE_DIR) && !rel.endsWith('/index.mdx')) {
		let cursor = 0;
		for (const section of FEATURE_SECTIONS) {
			const at = body.indexOf(`\n${section}\n`, cursor);
			if (at === -1) {
				problems.push(`${rel}: feature page is missing or misorders ${section}`);
				break;
			}
			cursor = at;
		}
	}
	if (rel.startsWith(CONTENT)) checkForm(rel, body);
}

/**
 * A paragraph or list item past MAX_PARAGRAPH_WORDS, and a `##` section past
 * MAX_SECTION_WORDS without a `###`, are the two shapes the writing rules name
 * as walls. Tables, JSX lines and headings end a block and are not counted.
 */
function checkForm(rel: string, body: string) {
	let block: { start: number; words: number } | undefined;
	let section: { start: number; title: string; words: number; sub: boolean } | undefined;
	const words = (l: string) => l.trim().split(/\s+/).filter(Boolean).length;
	const endBlock = () => {
		if (block && block.words > MAX_PARAGRAPH_WORDS) {
			problems.push(`${rel}:${block.start + 1}: paragraph of ${block.words} words (max ${MAX_PARAGRAPH_WORDS}) - split it, lead each part with its rule`);
		}
		block = undefined;
	};
	const endSection = () => {
		if (section && !section.sub && section.words > MAX_SECTION_WORDS) {
			problems.push(`${rel}:${section.start + 1}: section "${section.title}" has ${section.words} words and no ### (max ${MAX_SECTION_WORDS})`);
		}
		section = undefined;
	};
	for (const [line, i] of proseLines(body)) {
		const t = line.trim();
		if (/^##\s/.test(t)) {
			endBlock();
			endSection();
			section = { start: i, title: t.slice(3), words: 0, sub: false };
			continue;
		}
		if (/^###\s/.test(t) && section) section.sub = true;
		if (t === '' || /^(#|\||<|import\s)/.test(t)) {
			endBlock();
			continue;
		}
		if (/^([-*]|\d+\.)\s/.test(t)) endBlock();
		block ??= { start: i, words: 0 };
		block.words += words(t);
		if (section) section.words += words(t);
	}
	endBlock();
	endSection();
}

// Navigation is built from each folder's meta.json: a page it omits is
// unreachable from the sidebar, and a name it lists without a file is a dead
// entry.
for (const dir of [path.join(ROOT, CONTENT), ...readdirSync(path.join(ROOT, CONTENT), { withFileTypes: true }).filter((e) => e.isDirectory()).map((e) => path.join(ROOT, CONTENT, e.name))]) {
	const metaFile = path.join(dir, 'meta.json');
	const rel = path.relative(ROOT, metaFile);
	if (!existsSync(metaFile)) {
		problems.push(`${rel}: missing`);
		continue;
	}
	const listed = new Set<string>((JSON.parse(readFileSync(metaFile, 'utf8')).pages ?? []).filter((p: string) => !p.startsWith('---')));
	const present = readdirSync(dir, { withFileTypes: true })
		.filter((e) => e.isDirectory() || e.name.endsWith('.mdx'))
		.map((e) => e.name.replace(/\.mdx$/, ''));
	for (const name of present) if (!listed.has(name)) problems.push(`${rel}: does not list ${name}`);
	for (const name of listed) if (!present.includes(name)) problems.push(`${rel}: lists ${name}, which has no file`);
}

// The how-to index is the only list of how-tos an agent reads; a page missing
// from it is a page nobody finds, and a card whose text has drifted from the
// page's description is one fact written twice.
const howtoIndex = readFileSync(path.join(ROOT, HOWTO_INDEX), 'utf8');
for (const route of routes.keys()) {
	if (!route.startsWith('/howtos/')) continue;
	const card = howtoIndex.match(new RegExp(`<Card href="${route}"[^>]*description="([^"]*)"`));
	if (!card) {
		problems.push(`${HOWTO_INDEX}: no <Card> for ${route}`);
		continue;
	}
	const want = description(readFileSync(path.join(ROOT, CONTENT, `${route}.mdx`), 'utf8'));
	if (card[1] !== want) problems.push(`${HOWTO_INDEX}: card for ${route} says "${card[1]}", the page says "${want}"`);
}

if (problems.length > 0) {
	console.error(`memory-audit: ${problems.length} problem(s)\n`);
	for (const p of problems) console.error(`  ${p}`);
	console.error(`\nSee ${RULES_PAGE} for how the memory is written.`);
	process.exit(1);
}

console.log(`memory-audit: ${files.length} files, no stale tasks, paths or links, every page in navigation and in form`);
