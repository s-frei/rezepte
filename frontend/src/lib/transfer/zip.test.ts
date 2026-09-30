import { describe, expect, it, vi } from 'vitest';
import { strToU8, unzipSync, zipSync, type UnzipOptions } from 'fflate';
import { normalizeTitle, packFolder, readExport } from './zip';

// Records every entry the code under test lets fflate inflate.
const inflated: string[] = [];
vi.mock('fflate', async (importOriginal) => {
	const real = await importOriginal<typeof import('fflate')>();
	return {
		...real,
		unzipSync: (data: Uint8Array, opts?: UnzipOptions) =>
			real.unzipSync(data, {
				filter: (f) => {
					const keep = opts?.filter ? opts.filter(f) : true;
					if (keep) inflated.push(f.name);
					return keep;
				}
			})
	};
});

const recipe = (over: Record<string, unknown> = {}) =>
	strToU8(
		JSON.stringify({
			format: 'rezepte.recipe',
			version: 1,
			title: 'Focaccia',
			tags: ['bread'],
			images: ['1.jpg', '2.jpg'],
			cover: '2.jpg',
			...over
		})
	);

describe('readExport', () => {
	it('reads one row per folder, with the cover bytes', () => {
		const zip = zipSync({
			'focaccia/recipe.json': recipe(),
			'focaccia/1.jpg': new Uint8Array([1]),
			'focaccia/2.jpg': new Uint8Array([2]),
			'__MACOSX/focaccia/._1.jpg': new Uint8Array([9])
		});
		const rows = readExport(zip);
		expect(rows).toEqual([
			{
				folder: 'focaccia',
				title: 'Focaccia',
				tags: ['bread'],
				imageCount: 2,
				cover: new Uint8Array([2]),
				error: null
			}
		]);
	});

	it('marks a newer version and an unknown format as not importable', () => {
		const zip = zipSync({
			'a/recipe.json': recipe({ version: 2 }),
			'b/recipe.json': recipe({ format: 'other' })
		});
		const rows = readExport(zip);
		expect(rows.map((r) => r.error)).toEqual(['newer-version', 'unknown-format']);
	});

	it('marks a missing, non-number or zero version as unsupported', () => {
		const zip = zipSync({
			'a/recipe.json': recipe({ version: undefined }),
			'b/recipe.json': recipe({ version: '1' }),
			'c/recipe.json': recipe({ version: 0 })
		});
		const rows = readExport(zip);
		expect(rows.map((r) => r.error)).toEqual([
			'unsupported-version',
			'unsupported-version',
			'unsupported-version'
		]);
	});

	it('inflates only recipe.json and the cover, and returns no other photo', () => {
		const zip = zipSync({
			'focaccia/recipe.json': recipe(),
			'focaccia/1.jpg': new Uint8Array([1]),
			'focaccia/2.jpg': new Uint8Array([2])
		});
		inflated.length = 0;
		const rows = readExport(zip);
		expect(inflated.sort()).toEqual(['focaccia/2.jpg', 'focaccia/recipe.json']);
		expect(Array.isArray(rows)).toBe(true);
		expect(Object.keys(rows[0]).sort()).toEqual([
			'cover',
			'error',
			'folder',
			'imageCount',
			'tags',
			'title'
		]);
		expect(rows[0].cover).toEqual(new Uint8Array([2]));
	});

	it('marks a recipe.json that is not an object as unreadable', () => {
		const zip = zipSync({
			'a/recipe.json': strToU8('null'),
			'b/recipe.json': strToU8('3'),
			'c/recipe.json': strToU8('{')
		});
		const rows = readExport(zip);
		expect(rows.map((r) => r.error)).toEqual(['unreadable', 'unreadable', 'unreadable']);
	});

	it('throws on a file that is not a zip', () => {
		expect(() => readExport(new Uint8Array([1, 2, 3]))).toThrow();
	});
});

describe('packFolder', () => {
	it('keeps only the folder, uncompressed', () => {
		const zip = zipSync({
			'a/recipe.json': recipe(),
			'a/1.jpg': new Uint8Array([1]),
			'ab/recipe.json': recipe(),
			'b/recipe.json': recipe()
		});
		const out = unzipSync(packFolder(zip, 'a'));
		expect(Object.keys(out).sort()).toEqual(['a/1.jpg', 'a/recipe.json']);
	});
});

describe('normalizeTitle', () => {
	it('ignores case and surrounding space', () => {
		expect(normalizeTitle('  Äpfel Kuchen ')).toBe(normalizeTitle('äpfel kuchen'));
	});
});
