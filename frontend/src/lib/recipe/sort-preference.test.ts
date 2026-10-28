import { describe, expect, it } from 'vitest';
import { parseListQuery } from './query';
import { applyRememberedSort } from './sort-preference';

const at = (search: string) => new URL(`https://rezepte.test/${search}`);

describe('applyRememberedSort', () => {
	it('opens with the remembered order when the address names none', () => {
		const url = at('?tags=soup');
		expect(applyRememberedSort(parseListQuery(url), url, 'title').sort).toBe('title');
	});

	it("keeps the address's own order, the default included", () => {
		const url = at('?sort=updated');
		expect(applyRememberedSort(parseListQuery(url), url, 'title').sort).toBe('updated');
	});

	it('changes nothing without a remembered order', () => {
		const url = at('');
		expect(applyRememberedSort(parseListQuery(url), url, null)).toEqual(parseListQuery(url));
	});
});
