import { describe, expect, it } from 'vitest';
import type { ShoppingItem, ShoppingList } from '#lib/api/shopping.js';
import { addedIds } from './added';

const item = (id: string, by: string) => ({ id, addedBy: { id: by } }) as ShoppingItem;
const list = (...items: ShoppingItem[]) => ({ items }) as ShoppingList;

describe('addedIds', () => {
	it('keeps only new items I added', () => {
		const before = list(item('a', 'me'));
		const after = list(item('a', 'me'), item('b', 'me'), item('c', 'mila'));
		expect(addedIds(before, after, 'me')).toEqual(['b']);
	});
});
