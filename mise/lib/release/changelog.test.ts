import { describe, expect, test } from 'bun:test';
import { checkPage, draftPage, insertIntoMeta, markdownUrl, pageUrl, parseFrontmatter, releaseBody } from './changelog';

const none = { breaking: [], features: [], other: [] };

const finished = `---
title: v1.1.0 — Steps point at their ingredients
description: Steps link their ingredients, and the API names people consistently.
date: 2026-10-02
upgrade: Rename \`login\` to \`username\` in API clients.
---

## Steps point at their ingredients

A step can mention the ingredients it uses.
`;

describe('parseFrontmatter', () => {
	test('reads the fields and normalizes the date', () => {
		expect(parseFrontmatter(finished)).toEqual({
			data: {
				title: 'v1.1.0 — Steps point at their ingredients',
				description: 'Steps link their ingredients, and the API names people consistently.',
				date: '2026-10-02',
				upgrade: 'Rename `login` to `username` in API clients.'
			}
		});
	});

	test('a page without frontmatter is an error', () => {
		expect(parseFrontmatter('## Just a heading\n')).toEqual({ error: 'no frontmatter block' });
	});

	test('invalid YAML is an error, not an exception', () => {
		const r = parseFrontmatter('---\ndescription: a: b: c\n  - x\n---\n');
		expect('error' in r).toBe(true);
	});
});

describe('draftPage', () => {
	test('carries the marker, placeholders and the commits as raw material', () => {
		const page = draftPage('v1.1.0', '2026-10-02', 'v1.0.0', {
			breaking: ['refactor(api)!: say username'],
			features: ['feat: link ingredients'],
			other: ['fix: keep lang in step']
		});
		expect(page).toContain('title: "v1.1.0 — FILL IN"');
		expect(page).toContain('description: "FILL IN"');
		expect(page).toContain('date: 2026-10-02');
		expect(page).toContain('upgrade: "FILL IN"');
		expect(page).toContain('{/* DRAFT */}');
		expect(page).toContain('Commits since v1.0.0');
		expect(page).toContain('- refactor(api)!: say username');
		expect(page).toContain('- feat: link ingredients');
		expect(page).toContain('- fix: keep lang in step');
	});

	test('has no upgrade line when nothing is breaking', () => {
		expect(draftPage('v1.0.1', '2026-10-02', 'v1.0.0', { ...none, other: ['fix: a'] })).not.toContain('upgrade:');
	});

	test('names the whole history for the first release', () => {
		expect(draftPage('v1.0.0', '2026-10-02', null, { ...none, other: ['chore: init'] })).toContain('All commits so far');
	});

	test('a subject containing */ cannot end the MDX comment', () => {
		const page = draftPage('v1.0.1', '2026-10-02', 'v1.0.0', { ...none, other: ['fix: match /*/ paths */ too'] });
		const comment = page.slice(page.indexOf('{/*\n'), page.indexOf('\n*/}'));
		expect(comment).not.toContain('*/');
	});

	test('the draft fails checkPage until it is written', () => {
		const page = draftPage('v1.1.0', '2026-10-02', 'v1.0.0', { ...none, breaking: ['x!: y'] });
		expect(checkPage(page, true)).toEqual(['the DRAFT marker is still there', '"FILL IN" is still there']);
	});
});

describe('insertIntoMeta', () => {
	test('puts the new version right after index', () => {
		const meta = '{\n\t"title": "Changelog",\n\t"pages": ["index", "v1.0.0"]\n}\n';
		expect(JSON.parse(insertIntoMeta(meta, 'v1.1.0')).pages).toEqual(['index', 'v1.1.0', 'v1.0.0']);
	});

	test('ends with a newline and tab indentation', () => {
		const out = insertIntoMeta('{"title":"Changelog","pages":["index"]}', 'v1.0.0');
		expect(out).toBe('{\n\t"title": "Changelog",\n\t"pages": [\n\t\t"index",\n\t\t"v1.0.0"\n\t]\n}\n');
	});

	test('refuses a version that is already listed', () => {
		expect(() => insertIntoMeta('{"pages":["index","v1.0.0"]}', 'v1.0.0')).toThrow('v1.0.0 is already in');
	});
});

describe('checkPage', () => {
	test('a finished page passes', () => {
		expect(checkPage(finished, true)).toEqual([]);
	});

	test('reports missing fields', () => {
		expect(checkPage('---\ntitle: v1.0.0 — First\n---\n\nBody\n', false)).toEqual([
			'frontmatter has no description',
			'frontmatter has no date'
		]);
	});

	test('breaking commits require an upgrade notice', () => {
		const page = finished.replace(/^upgrade: .*\n/m, '');
		expect(checkPage(page, true)).toEqual(['the release has breaking commits but no upgrade notice']);
		expect(checkPage(page, false)).toEqual([]);
	});

	test('broken YAML is reported, not thrown', () => {
		expect(checkPage('---\ndescription: a: b: c\n  - x\n---\n', false)[0]).toStartWith('frontmatter does not parse');
	});
});

describe('release body', () => {
	test('urls follow the docs site', () => {
		expect(pageUrl('v1.1.0')).toBe('https://s-frei.github.io/rezepte/changelog/v1.1.0/');
		expect(markdownUrl('v1.1.0')).toBe('https://s-frei.github.io/rezepte/llms.mdx/changelog/v1.1.0/content.md');
	});

	test('with an upgrade notice', () => {
		expect(
			releaseBody('v1.1.0', { description: 'Steps link their ingredients.', upgrade: 'Rename `login` to `username`.' })
		).toBe(
			[
				'**Before you upgrade:** Rename `login` to `username`.',
				'',
				'Steps link their ingredients.',
				'',
				'📖 Release notes: https://s-frei.github.io/rezepte/changelog/v1.1.0/',
				'🤖 For LLMs: https://s-frei.github.io/rezepte/llms.mdx/changelog/v1.1.0/content.md',
				''
			].join('\n')
		);
	});

	test('without an upgrade notice', () => {
		expect(releaseBody('v1.0.1', { description: 'Fixes.' })).toStartWith('Fixes.\n\n📖');
	});
});
