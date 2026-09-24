// Fails the build when the agent memory claims something the repository does
// not support. It checks the two kinds of claim that rot silently - a task or
// a path that was renamed or deleted - plus the one kind that must never be
// written at all.
//
// Deliberately narrow. It cannot tell whether a sentence is true, only whether
// the things it names still exist, so it catches renames and deletions and
// nothing else. Prose drift is the writing rules' job (see content/index.mdx),
// not this script's.
//
// Ports are deliberately NOT checked. Every port in the memory is either a
// command example carrying a `mise run ports` pointer or, in the writing rules
// on content/index.mdx, a rejected one named as the reason it is rejected. A
// check would need more exemptions than it has findings.
import { readdirSync, readFileSync, existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execSync } from 'node:child_process';

// Resolved from this file rather than the working directory, so the script
// behaves the same however it is invoked. `next build` type-checks it with the
// site's tsconfig, which has no Bun globals - hence fileURLToPath, not
// import.meta.dir.
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..');
const SOURCES = ['docs/memory/content', 'AGENTS.md'];

/** Phrasings that tie the memory to one person's machine rather than to the repository. */
const MACHINE_INTERNALS = [
	/\bdevelopment (machine|Mac|laptop)\b/i,
	/\bon (this|my) (machine|Mac|laptop)\b/i,
	/\/Users\/[a-z]/i,
	/\bDocker Desktop\b/,
	/\bscreen is locked\b/i
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
	/\b20[0-9]{2}-[01][0-9]-[0-9]{2}\b/
];

/** Paths that are not expected to resolve, and why. */
function pathIsExempt(p: string): boolean {
	if (p.includes('*')) return true; // a glob, not a file
	if (p.includes('NNNN')) return true; // a naming template
	if (/\/[a-z]+\.[A-Z]/.test(p)) return true; // a Go symbol such as demo.Seed
	// Gitignored artifacts are named on purpose (local agent scratch space,
	// static exports, build output, data dirs); they are absent from a clean
	// checkout.
	return /^(docs\/superpowers|docs\/design_handoff_rezepte|docs\/memory\/out|docs\/user\/out|frontend\/test-results|service\/bin|data)(\/|$)/.test(
		p
	);
}

/** A placeholder standing in for a real name, not a claim that one exists. */
function isPlaceholder(s: string): boolean {
	return /[…<>]/.test(s) || s.includes('NNNN');
}

function walk(dir: string): string[] {
	return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
		const p = path.join(dir, e.name);
		return e.isDirectory() ? walk(p) : p.endsWith('.mdx') ? [p] : [];
	});
}

/**
 * Every page in the memory, addressed the way a link addresses it:
 * content/features/recipes.mdx -> /features/recipes, content/index.mdx -> /,
 * content/howtos/index.mdx -> /howtos. A link into the memory that resolves to
 * nothing is a dead end for the agent that follows it, and renaming a page is
 * exactly when one appears.
 */
function memoryRoutes(): Set<string> {
	const base = path.join(ROOT, 'docs/memory/content');
	const routes = new Set<string>();
	for (const file of walk(base)) {
		const rel = path.relative(base, file).replace(/\.mdx$/, '');
		const route = rel === 'index' ? '/' : '/' + rel.replace(/\/index$/, '');
		routes.add(route);
	}
	return routes;
}

/** The sections every features/ page carries, in the order they must appear. */
const FEATURE_SECTIONS = ['## How it works', '## Why it is this way', '## Rejected', '## Limits'];

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

const problems: string[] = [];

for (const file of files) {
	const rel = path.relative(ROOT, file);
	const body = readFileSync(file, 'utf8');
	body
		.split('\n')
		.forEach((line, i) => {
			const at = `${rel}:${i + 1}`;

			// content/index.mdx states the writing rules by quoting what they forbid.
			const statesTheRule = rel.endsWith('content/index.mdx');

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
				const task = m[1].split(/\s/)[0];
				if (isPlaceholder(task)) continue;
				if (!tasks.has(task) && !tasks.has(`//${task}`)) {
					problems.push(`${at}: no such mise task: ${task}`);
				}
			}

			// The four package directories, `mise/` and `.github/`, plus the two
			// root files the memory names by name - a path check that skipped those
			// would leave the most-cited files in the repository unverified.
			// `scripts/` stays in the list although no such directory exists: a
			// page that still points at one has to fail rather than read as true.
			for (const m of line.matchAll(
				/`((?:service|frontend|docs|scripts|mise|\.github)\/[A-Za-z0-9_./*{}-]+|mise\.toml|CLAUDE\.md)`/g
			)) {
				const p = m[1];
				if (!pathIsExempt(p) && !existsSync(path.join(ROOT, p))) {
					problems.push(`${at}: path does not exist: ${p}`);
				}
			}

			// The memory's how-to index addresses pages with JSX (`<Card
			// href="...">`) rather than markdown links, so both forms are checked
			// against the same route set.
			for (const m of line.matchAll(/\]\((\/[A-Za-z0-9/._#?-]*)\)|href="(\/[A-Za-z0-9/._#?-]*)"/g)) {
				const target = (m[1] ?? m[2]).replace(/[#?].*$/, '').replace(/\/$/, '') || '/';
				if (!routes.has(target)) {
					problems.push(`${at}: link goes nowhere: ${target}`);
				}
			}
		});

	if (rel.includes('content/features/') && !rel.endsWith('/index.mdx')) {
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
}

if (problems.length > 0) {
	console.error(`memory-audit: ${problems.length} problem(s)\n`);
	for (const p of problems) console.error(`  ${p}`);
	console.error(`\nSee docs/memory/content/index.mdx for what belongs in the memory.`);
	process.exit(1);
}

console.log(`memory-audit: ${files.length} files, no stale tasks or paths, no machine internals`);
