import { describe, expect, it, vi } from 'vitest';
import type { ShoppingList } from '#lib/api/shopping.js';
import { ListState } from './list-state.svelte';

const make = (checked = false, id = 'l1') =>
	({ id, name: 'L', items: [{ id: 'i1', checked }], entries: [] }) as unknown as ShoppingList;

function deferred<T>() {
	let resolve!: (v: T) => void;
	let reject!: (e: unknown) => void;
	const promise = new Promise<T>((res, rej) => ((resolve = res), (reject = rej)));
	return { promise, resolve, reject };
}

describe('ListState', () => {
	it('refresh drops its answer when a local change started meanwhile', async () => {
		const get = deferred<ShoppingList>();
		const state = new ListState(make(), vi.fn(), () => get.promise);
		const refreshing = state.refresh();
		state.list = make(true);
		const done = state.apply(Promise.resolve(make(true)));
		get.resolve(make(false));
		await refreshing;
		await done;
		expect(state.list.items[0].checked).toBe(true);
	});

	it('apply keeps only the newest answer', async () => {
		const state = new ListState(make(), vi.fn());
		const first = deferred<ShoppingList>();
		const a = state.apply(first.promise);
		const b = state.apply(Promise.resolve(make(true)));
		await b;
		first.resolve(make(false));
		await a;
		expect(state.list.items[0].checked).toBe(true);
	});

	it('a failed apply reverts only when nothing newer was sent', async () => {
		const onfail = vi.fn();
		const state = new ListState(make(), onfail, () => Promise.resolve(make(true)));
		state.list = make(true);
		expect(await state.apply(Promise.reject(new Error('x')), () => make(false))).toBe(false);
		expect(state.list.items[0].checked).toBe(false);
		expect(onfail).toHaveBeenCalledTimes(1);

		// A newer change in flight: no local revert, a reload instead.
		const slow = deferred<ShoppingList>();
		const failing = deferred<ShoppingList>();
		const a = state.apply(failing.promise, () => make(false));
		const b = state.apply(slow.promise);
		failing.reject(new Error('x'));
		await a;
		slow.resolve(make(true));
		await b;
		expect(state.list.items[0].checked).toBe(true);
	});

	it('load takes a new list and drops answers for the old one', async () => {
		const state = new ListState(make(), vi.fn());
		const old = deferred<ShoppingList>();
		const a = state.apply(old.promise);
		state.load(make(false, 'l2'));
		old.resolve(make(true));
		await a;
		expect(state.list.id).toBe('l2');
	});
});
