import { describe, expect, it } from 'vitest';
import { DEFAULT_SORT, parseListQuery } from './query';
import { applyRememberedSort } from './sort-preference';

const at = (search: string) => new URL(`https://rezepte.test/${search}`);

describe('applyRememberedSort', () => {
	it('opens with the remembered order when the address names none', () => {
		const url = at('?tags=soup');
		expect(applyRememberedSort(parseListQuery(url), url, 'title').sort).toBe('title');
	});

	it("keeps the address's own order", () => {
		const url = at('?sort=updated');
		expect(applyRememberedSort(parseListQuery(url), url, 'title').sort).toBe('updated');
	});

	it('changes nothing without a remembered order', () => {
		const url = at('');
		expect(applyRememberedSort(parseListQuery(url), url, null)).toEqual(parseListQuery(url));
	});
});

describe('applyRememberedSort with ingredient chips', () => {
	it('ignores the remembered order while the address has chips and no sort', () => {
		const url = at('?ingredients=Gurke');
		expect(applyRememberedSort(parseListQuery(url), url, 'title').sort).toBe(DEFAULT_SORT);
	});

	it('keeps an order the address names next to chips', () => {
		const url = at('?ingredients=Gurke&sort=title');
		expect(applyRememberedSort(parseListQuery(url), url, 'tasty').sort).toBe('title');
	});

	it('still applies the remembered order without chips', () => {
		const url = at('?have=Salz');
		expect(applyRememberedSort(parseListQuery(url), url, 'title').sort).toBe('title');
	});
});
