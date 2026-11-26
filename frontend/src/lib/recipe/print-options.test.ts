import { beforeEach, describe, expect, it, vi } from 'vitest';

const store = new Map<string, string>();
vi.mock('#lib/storage.js', () => ({
	load: (k: string) => store.get(k) ?? null,
	save: (k: string, v: string | null) => (v === null ? store.delete(k) : store.set(k, v))
}));

const defaults = { format: 'a4', photo: true, footer: true, bundle: 'book', servings: null };

async function fresh() {
	vi.resetModules();
	return import('./print-options.svelte');
}

beforeEach(() => store.clear());

describe('printOptions', () => {
	it('uses defaults for empty, garbage and invalid storage', async () => {
		expect({ ...(await fresh()).printOptions }).toEqual(defaults);
		store.set('rezepte-print', 'not json');
		expect({ ...(await fresh()).printOptions }).toEqual(defaults);
		store.set(
			'rezepte-print',
			JSON.stringify({ format: 'a3', photo: 'x', bundle: 1, servings: -2 })
		);
		expect({ ...(await fresh()).printOptions }).toEqual(defaults);
	});

	it('persists a change across loads', async () => {
		const first = await fresh();
		first.setPrintOption('format', 'a5');
		first.setPrintOption('servings', 6);
		first.setPrintOption('bundle', 'zip');
		const again = await fresh();
		expect({ ...again.printOptions }).toEqual({
			...defaults,
			format: 'a5',
			servings: 6,
			bundle: 'zip'
		});
	});
});
