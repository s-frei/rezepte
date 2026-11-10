import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest';
import {
	SHADOW_ITEM_MARKER_PROPERTY_NAME,
	SOURCES,
	TRIGGERS,
	setAriaStrings,
	type DndEvent
} from 'svelte-dnd-action';
import { applyDndAriaStrings, applyDrag, isShadow, pickUp, slotHeight } from './dnd';

describe('applyDndAriaStrings', () => {
	// setAriaStrings validates its argument and throws on an unknown key or a
	// value of the wrong type, so this pins our override object to the
	// library's contract - the part most likely to rot on an upgrade.
	it('is accepted by svelte-dnd-action', () => {
		expect(() => applyDndAriaStrings()).not.toThrow();
	});

	it('only applies once, since the setting is global', () => {
		expect(() => applyDndAriaStrings()).not.toThrow();
	});

	it('would reject a bad override, proving the check above has teeth', () => {
		expect(() => setAriaStrings({ notAThing: 'x' } as never)).toThrow();
		expect(() => setAriaStrings({ dropped: 'not a function' } as never)).toThrow();
	});
});

describe('drop slot', () => {
	it('tells the shadow item from the others', () => {
		expect(isShadow({ id: 'a', [SHADOW_ITEM_MARKER_PROPERTY_NAME]: true })).toBe(true);
		expect(isShadow({ id: 'a' })).toBe(false);
	});

	it('keeps the height read at drag start through every later event', () => {
		const event = (trigger: TRIGGERS) =>
			new CustomEvent<DndEvent<unknown>>('consider', {
				detail: { items: [], info: { id: 'a', trigger, source: SOURCES.POINTER } }
			});
		expect(slotHeight(event(TRIGGERS.DRAGGED_OVER_INDEX))).toBeNull();
		expect(slotHeight(event(TRIGGERS.DROPPED_INTO_ZONE))).toBeNull();
	});
});

describe('applyDrag', () => {
	type Item = { id: string };
	const focused: string[] = [];
	const flashed: string[] = [];
	// Node has no DOM: the document only needs to take listeners and name
	// the focused element. The lists are event targets whose items answer
	// the handle query with their position.
	const page = Object.assign(new EventTarget(), {
		activeElement: null as unknown,
		body: { tag: 'body' }
	});
	beforeAll(() => {
		globalThis.document = page as unknown as Document;
		globalThis.matchMedia = (() => ({ matches: false })) as unknown as typeof matchMedia;
	});
	afterAll(() => {
		delete (globalThis as { document?: Document }).document;
		delete (globalThis as { matchMedia?: unknown }).matchMedia;
	});
	const zone = (name: string, items: Item[]) => {
		const slot = (index: number) => ({
			querySelector: () => ({ focus: () => focused.push(`${name}[${index}]`) }),
			animate: () => flashed.push(`${name}[${index}]`)
		});
		const state = {
			items,
			list: Object.assign(new EventTarget(), { children: [0, 1, 2].map(slot) })
		};
		state.list.addEventListener('consider', (event) =>
			applyDrag(event as CustomEvent<DndEvent<Item>>, state.items, (next) => (state.items = next))
		);
		return state;
	};
	const send = (target: EventTarget, items: Item[], trigger: TRIGGERS, source = SOURCES.KEYBOARD) =>
		target.dispatchEvent(
			new CustomEvent('consider', { detail: { items, info: { id: 'a', trigger, source } } })
		);
	const settle = () => new Promise((resolve) => setTimeout(resolve));
	const ids = (items: Item[]) => items.map((item) => item.id).join('');
	const [a, b, c, x] = ['a', 'b', 'c', 'x'].map((id) => ({ id }));

	it('focuses the handle where a keyboard drop left the item', async () => {
		focused.length = 0;
		const one = zone('one', [a, b, c]);
		send(one.list, [a, b, c], TRIGGERS.DRAG_STARTED);
		send(one.list, [b, a, c], TRIGGERS.DROPPED_INTO_ZONE);
		send(one.list, [b, a, c], TRIGGERS.DRAG_STOPPED);
		await settle();
		expect(ids(one.items)).toBe('bac');
		expect(focused).toEqual(['one[1]']);
	});

	it('lights up where a keyboard drop placed the item, not after Escape', async () => {
		flashed.length = 0;
		const one = zone('one', [a, b, c]);
		send(one.list, [a, b, c], TRIGGERS.DRAG_STARTED);
		send(one.list, [b, a, c], TRIGGERS.DRAG_STOPPED);
		send(one.list, [b, a, c], TRIGGERS.DRAG_STARTED);
		page.dispatchEvent(Object.assign(new Event('keydown'), { key: 'Escape' }));
		send(one.list, [b, c, a], TRIGGERS.DRAG_STOPPED);
		await settle();
		expect(flashed).toEqual(['one[1]']);
	});

	it('lights up where a pointer drop placed the item, only on the drop', async () => {
		flashed.length = 0;
		const one = zone('one', [a, b, c]);
		send(one.list, [b, a, c], TRIGGERS.DRAGGED_OVER_INDEX, SOURCES.POINTER);
		send(one.list, [b, c, a], TRIGGERS.DROPPED_INTO_ZONE, SOURCES.POINTER);
		await settle();
		expect(flashed).toEqual(['one[2]']);
	});

	it('leaves the focus alone when a click elsewhere ended the drag', async () => {
		focused.length = 0;
		const one = zone('one', [a, b, c]);
		send(one.list, [a, b, c], TRIGGERS.DRAG_STARTED);
		send(one.list, [b, a, c], TRIGGERS.DROPPED_INTO_ZONE);
		page.activeElement = { tag: 'input' };
		send(one.list, [b, a, c], TRIGGERS.DRAG_STOPPED);
		page.activeElement = null;
		await settle();
		expect(ids(one.items)).toBe('bac');
		expect(focused).toEqual([]);
	});

	it('puts every list back and focuses the starting handle on Escape', async () => {
		focused.length = 0;
		const one = zone('one', [b, a, c]);
		const two = zone('two', [x]);
		send(one.list, [b, a, c], TRIGGERS.DRAG_STARTED);
		send(one.list, [b, c, a], TRIGGERS.DROPPED_INTO_ZONE);
		// Tabbed into the other group.
		send(one.list, [b, c], TRIGGERS.DROPPED_INTO_ANOTHER);
		send(two.list, [a, x], TRIGGERS.DROPPED_INTO_ZONE);
		page.dispatchEvent(Object.assign(new Event('keydown'), { key: 'Escape' }));
		send(two.list, [a, x], TRIGGERS.DRAG_STOPPED);
		await settle();
		expect([ids(one.items), ids(two.items)]).toEqual(['bac', 'x']);
		expect(focused).toEqual(['one[1]']);
	});

	it('passes a pointer drag straight through', () => {
		const one = zone('one', [a, b, c]);
		send(one.list, [b, a, c], TRIGGERS.DRAG_STOPPED, SOURCES.POINTER);
		expect(ids(one.items)).toBe('bac');
	});
});

describe('pickUp', () => {
	// Node has no DOM: the window is an event target, the item a dataset.
	const press = (button = 0) => {
		const item = { dataset: {} as DOMStringMap };
		const handle = { closest: () => item };
		pickUp({ button, pointerType: 'touch', currentTarget: handle } as unknown as PointerEvent);
		return item.dataset;
	};
	beforeAll(() => {
		vi.useFakeTimers();
		const window = new EventTarget();
		vi.stubGlobal('addEventListener', window.addEventListener.bind(window));
		vi.stubGlobal('removeEventListener', window.removeEventListener.bind(window));
		vi.stubGlobal('dispatchEvent', window.dispatchEvent.bind(window));
	});
	afterAll(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	it('lifts on the press and settles back after the release', () => {
		const lift = press();
		expect(lift.lift).toBe('up');
		dispatchEvent(new Event('pointerup'));
		expect(lift.lift).toBe('down');
		vi.advanceTimersByTime(160);
		expect(lift.lift).toBeUndefined();
	});

	it('stays up when pressed again while it settles', () => {
		const first = press();
		dispatchEvent(new Event('pointerup'));
		first.lift = 'up';
		vi.advanceTimersByTime(160);
		expect(first.lift).toBe('up');
	});

	it('ignores any button but the main one', () => {
		expect(press(2).lift).toBeUndefined();
	});
});
