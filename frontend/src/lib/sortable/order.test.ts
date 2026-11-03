import { describe, expect, test } from 'vitest';
import { keyTarget, moveItem } from './order.js';

describe('moveItem', () => {
	test('moves an item to a new index', () => {
		expect(moveItem(['a', 'b', 'c', 'd'], 3, 0)).toEqual(['d', 'a', 'b', 'c']);
		expect(moveItem(['a', 'b', 'c', 'd'], 0, 2)).toEqual(['b', 'c', 'a', 'd']);
	});

	test('clamps the target to the last index', () => {
		expect(moveItem(['a', 'b', 'c', 'd'], 0, 9)).toEqual(['b', 'c', 'd', 'a']);
	});

	test('leaves the input untouched', () => {
		const list = ['a', 'b'];
		moveItem(list, 0, 1);
		expect(list).toEqual(['a', 'b']);
	});
});

describe('keyTarget', () => {
	test('left and up move one place earlier', () => {
		expect(keyTarget(4, 8, 'ArrowLeft')).toBe(3);
		expect(keyTarget(4, 8, 'ArrowUp')).toBe(3);
	});

	test('right and down move one place later', () => {
		expect(keyTarget(4, 8, 'ArrowRight')).toBe(5);
		expect(keyTarget(4, 8, 'ArrowDown')).toBe(5);
	});

	test('stays at an end', () => {
		expect(keyTarget(0, 8, 'ArrowLeft')).toBe(0);
		expect(keyTarget(7, 8, 'ArrowDown')).toBe(7);
	});

	test('returns null for any other key', () => {
		expect(keyTarget(2, 8, 'Enter')).toBeNull();
	});
});
