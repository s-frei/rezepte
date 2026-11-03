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
	test('up and down move by a row', () => {
		expect(keyTarget(4, 8, 'ArrowUp', 3)).toBe(1);
		expect(keyTarget(4, 8, 'ArrowDown', 3)).toBe(7);
	});

	test('left and right move by one', () => {
		expect(keyTarget(4, 8, 'ArrowLeft', 3)).toBe(3);
		expect(keyTarget(4, 8, 'ArrowRight', 3)).toBe(5);
	});

	test('clamps a row step to the ends', () => {
		expect(keyTarget(1, 8, 'ArrowUp', 3)).toBe(0);
		expect(keyTarget(6, 8, 'ArrowDown', 3)).toBe(7);
	});

	test('returns null at an edge', () => {
		expect(keyTarget(0, 8, 'ArrowLeft', 3)).toBeNull();
		expect(keyTarget(7, 8, 'ArrowRight', 3)).toBeNull();
	});

	test('returns null for any other key', () => {
		expect(keyTarget(2, 8, 'Enter', 3)).toBeNull();
	});
});
