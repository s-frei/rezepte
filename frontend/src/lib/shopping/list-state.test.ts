import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '#lib/api/client.js';
import { getShoppingList, setItemsChecked, type ShoppingList } from '#lib/api/shopping.js';
import { LINGER_MS, ListState } from './list-state.svelte';

vi.mock('#lib/api/shopping.js', () => ({ getShoppingList: vi.fn(), setItemsChecked: vi.fn() }));
const get = vi.mocked(getShoppingList);
const check = vi.mocked(setItemsChecked);

const make = (checked = false, id = 'l1') =>
	({
		id,
		name: 'L',
		revision: 0,
		items: [{ id: 'i1', checked }],
		entries: []
	}) as unknown as ShoppingList;

function deferred<T>() {
	let resolve!: (v: T) => void;
	let reject!: (e: unknown) => void;
	const promise = new Promise<T>((res, rej) => ((resolve = res), (reject = rej)));
	return { promise, resolve, reject };
}

describe('ListState', () => {
	beforeEach(() => vi.resetAllMocks());

	it('refresh drops its answer when a local change started meanwhile', async () => {
		const answer = deferred<ShoppingList>();
		get.mockReturnValue(answer.promise);
		const state = new ListState(make(), vi.fn());
		const refreshing = state.refresh();
		state.list = make(true);
		const done = state.apply(Promise.resolve(make(true)));
		answer.resolve(make(false));
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
		get.mockResolvedValue(make(true));
		const state = new ListState(make(), onfail);
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

	it('refresh reports only a gone list or a lost session, not a passing failure', async () => {
		const onfail = vi.fn();
		const state = new ListState(make(), onfail);
		get.mockRejectedValueOnce(new ApiError(500, {}));
		await state.refresh();
		get.mockRejectedValueOnce(new TypeError('offline'));
		await state.refresh();
		expect(onfail).not.toHaveBeenCalled();
		get.mockRejectedValueOnce(new ApiError(404, {}));
		await state.refresh();
		get.mockRejectedValueOnce(new ApiError(401, {}));
		await state.refresh();
		expect(onfail).toHaveBeenCalledTimes(2);
	});

	it('toggle flips the row at once and back when the request fails', async () => {
		const onfail = vi.fn();
		const state = new ListState(make(), onfail);
		const request = deferred<ShoppingList>();
		check.mockReturnValue(request.promise);
		const done = state.toggle({ checked: false, itemIds: ['i1'] });
		expect(state.list.items[0].checked).toBe(true);
		expect(check).toHaveBeenCalledWith('l1', ['i1'], true);
		request.reject(new Error('x'));
		expect(await done).toBe(false);
		expect(state.list.items[0].checked).toBe(false);
		expect(onfail).toHaveBeenCalledTimes(1);
	});
});

describe('ListState live', () => {
	beforeEach(() => vi.resetAllMocks());

	const at = (revision: number, checked = false) =>
		({ ...make(checked), revision }) as ShoppingList;

	it('follow skips the revision it shows already', async () => {
		get.mockResolvedValue(at(4));
		const state = new ListState(at(3), vi.fn());
		await state.follow(3);
		expect(get).not.toHaveBeenCalled();
		await state.follow(4);
		await state.follow();
		expect(get).toHaveBeenCalledTimes(2);
	});

	it('an item ticked elsewhere lingers open for a moment', async () => {
		vi.useFakeTimers();
		get.mockResolvedValue(at(4, true));
		const state = new ListState(at(3), vi.fn());
		await state.follow(4);
		expect(state.list.items[0].checked).toBe(true);
		expect(state.lingering).toEqual(['i1']);
		expect(state.shown.items[0].checked).toBe(false);
		vi.advanceTimersByTime(LINGER_MS);
		expect(state.lingering).toEqual([]);
		expect(state.shown.items[0].checked).toBe(true);
		vi.useRealTimers();
	});

	it('drops an answer older than the list shown', async () => {
		// A refetch brings revision 7 before the slow answer to one's own change (revision 6).
		const slow = deferred<ShoppingList>();
		const state = new ListState(at(5), vi.fn());
		const change = state.apply(slow.promise);
		get.mockResolvedValue(at(7, true));
		await state.follow(7);
		slow.resolve(at(6));
		await change;
		expect(state.list.revision).toBe(7);
	});

	it('unticking a lingering row ends the linger', async () => {
		get.mockResolvedValue(at(4, true));
		check.mockResolvedValue(at(5));
		const state = new ListState(at(3), vi.fn());
		await state.follow(4);
		expect(state.lingering).toEqual(['i1']);
		await state.toggle({ checked: true, itemIds: ['i1'] });
		expect(state.lingering).toEqual([]);
		expect(state.shown.items[0].checked).toBe(false);
	});

	it('an own tick does not linger', async () => {
		check.mockResolvedValue(at(4, true));
		const state = new ListState(at(3), vi.fn());
		await state.toggle({ checked: false, itemIds: ['i1'] });
		expect(state.lingering).toEqual([]);
	});
});
