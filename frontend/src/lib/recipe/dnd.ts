import {
	DRAGGED_ELEMENT_ID,
	SHADOW_ITEM_MARKER_PROPERTY_NAME,
	SOURCES,
	TRIGGERS,
	setAriaStrings,
	type DndEvent
} from 'svelte-dnd-action';
import { tick } from 'svelte';
import { m } from '#lib/paraglide/messages.js';

/**
 * svelte-dnd-action speaks to screen readers itself, out of its own built-in
 * strings. This hands it the app's copy instead, in whichever language the
 * page is rendering.
 *
 * The setting is global to the document and applies to every dnd zone, so it
 * only has to happen once - `applied` keeps repeated editor mounts from
 * redoing the work. `setAriaStrings` is a no-op on the server, so importing
 * this module is safe either way.
 */
let applied = false;

const KEY_PHRASES = {
	space: () => m.dnd_key_space(),
	enter: () => m.dnd_key_enter(),
	space_or_enter: () => m.dnd_key_space_or_enter()
};

export function applyDndAriaStrings(): void {
	if (applied) {
		return;
	}
	applied = true;
	setAriaStrings({
		// Two full sentences rather than one plus an appended clause: German
		// word order doesn't survive gluing message fragments together.
		dragStarted: ({ itemLabel, zoneLabel, canMoveBetweenZones }) =>
			canMoveBetweenZones
				? m.dnd_drag_started_multi({ item: itemLabel, zone: zoneLabel })
				: m.dnd_drag_started({ item: itemLabel, zone: zoneLabel }),
		movedToPosition: ({ itemLabel, zoneLabel, position, count }) =>
			m.dnd_moved_to_position({ item: itemLabel, zone: zoneLabel, position, count }),
		movedToZoneEnd: ({ itemLabel, zoneLabel }) =>
			m.dnd_moved_to_zone_end({ item: itemLabel, zone: zoneLabel }),
		movedToZoneStart: ({ itemLabel, zoneLabel }) =>
			m.dnd_moved_to_zone_start({ item: itemLabel, zone: zoneLabel }),
		dropped: ({ itemLabel, zoneLabel, position, count }) =>
			m.dnd_dropped({ item: itemLabel, zone: zoneLabel, position, count }),
		zoneActiveInstruction: ({ keyboardDragTrigger }) =>
			m.dnd_zone_instruction({ key: KEY_PHRASES[keyboardDragTrigger]() }),
		zoneDragDisabledInstruction: m.dnd_zone_disabled_instruction()
	});
}

/**
 * The item svelte-dnd-action puts in the list where a pointer drag will
 * land. The editor's lists draw it as a drop slot instead of the item.
 */
export function isShadow(item: object): boolean {
	return (item as Record<string, unknown>)[SHADOW_ITEM_MARKER_PROPERTY_NAME] === true;
}

/**
 * The height the drop slot takes, read when a pointer drag starts: that of
 * the copy the library drags, which it sizes to the dragged item. `null` for
 * any other event, which keeps the height read at the start.
 *
 * The slot cannot measure itself. The library swaps the dragged item for a
 * shadow under another id, so the list mounts a new element for it, and it
 * sizes the dragged copy to that element in the same moment. A step mounted
 * there was 0px tall until its editor had mounted, so the copy collapsed and
 * jumped away from the pointer. A slot of a known height is the right size
 * from its first frame.
 */
export function slotHeight(event: CustomEvent<DndEvent<unknown>>): number | null {
	if (event.detail.info.trigger !== TRIGGERS.DRAG_STARTED) return null;
	return document.getElementById(DRAGGED_ELEMENT_ID)?.getBoundingClientRect().height ?? null;
}

/**
 * A drag handle's `onpointerdown`: picks its item up the moment it is
 * pressed, before svelte-dnd-action starts a drag (it waits for the pointer
 * to move 3px). The item is marked `data-lift="up"`, which the lists draw as
 * a card - the surface, `--shadow-lift` and the primary outline - growing 8px
 * out of the item over 160ms ease-out, and a finger feels a 10ms buzz: the
 * photos' pickup (`#lib/sortable`). The copy the library drags is cloned from
 * the item, so it carries the card from its first frame, at the item's own
 * size and place. Let go without a drag, the card settles back
 * (`data-lift="down"`, which keeps the item in front while it does) and the
 * mark goes. Drawn in CSS, not scaled: a full-width row scaled like a photo
 * would run past the screen's edge and blur its text.
 */
export function pickUp(event: PointerEvent): void {
	if (event.button !== 0) return;
	const item = (event.currentTarget as Element).closest('li');
	if (!item) return;
	item.dataset.lift = 'up';
	if (event.pointerType !== 'mouse') navigator.vibrate?.(10);
	const settle = () => {
		removeEventListener('pointerup', settle);
		removeEventListener('pointercancel', settle);
		if (item.dataset.lift !== 'up') return;
		item.dataset.lift = 'down';
		setTimeout(() => {
			if (item.dataset.lift === 'down') delete item.dataset.lift;
		}, 160);
	};
	addEventListener('pointerup', settle);
	addEventListener('pointercancel', settle);
}

/**
 * A zone's `transformDraggedElement`: keeps the dragged copy picked up even
 * if its press was missed (`pickUp`). Needs `morphDisabled`, or the library
 * sizes the copy to the slot under it.
 */
export function liftDragged(element?: HTMLElement): void {
	if (element) element.dataset.lift = 'up';
}

/**
 * What both editor lists hand `dragHandleZone`: no drop-target outline, the
 * list itself no Tab stop (the handles are the keyboard way in), and the
 * dragged copy kept picked up (`liftDragged`) rather than morphed.
 */
export function zoneOptions<T extends { id: unknown }>(
	items: T[],
	type: string,
	flipDurationMs: number
) {
	return {
		items,
		type,
		flipDurationMs,
		dropTargetStyle: {},
		zoneTabIndex: -1,
		morphDisabled: true,
		transformDraggedElement: liftDragged
	};
}

/**
 * Lights up the item a drag just placed: the primary outline the lifted
 * copy wore, with the accent behind it, fading out over 800ms - so the eye
 * finds where it landed. The outline stands `--flash-offset` (4px) off the
 * item; a list with less room around its items sets it closer. A Web
 * Animation rather than a class: it leaves nothing behind on the element
 * once it has run. Under reduced motion the light holds as long and goes
 * out at once instead of fading.
 */
export function flashPlaced(element: Element | undefined): void {
	if (!element?.animate) return;
	const lit = {
		outline: '2px solid var(--color-primary)',
		outlineOffset: 'var(--flash-offset, 4px)',
		borderRadius: 'var(--radius-lg)',
		background: 'color-mix(in oklab, var(--color-accent) 60%, transparent)'
	};
	const still = matchMedia('(prefers-reduced-motion: reduce)').matches;
	element.animate(
		[
			lit,
			{ ...lit, offset: 0.3 },
			{ ...lit, outlineColor: 'transparent', background: 'transparent' }
		],
		{ duration: 800, easing: still ? 'step-end' : 'cubic-bezier(0.4, 0, 0.2, 1)' }
	);
}

/** The keyboard drag under way: what Escape puts back, and where the item started. */
let keyboardDrag: {
	restore: Map<EventTarget, () => void>;
	origin: { list: Element; index: number };
	escaped: boolean;
} | null = null;

/**
 * Runs in the capture phase, ahead of the library's own Escape handler on
 * the window, which drops the item and tells no one the drag was canceled.
 */
function noteEscape(event: Event) {
	if (keyboardDrag && (event as KeyboardEvent).key === 'Escape') keyboardDrag.escaped = true;
}

/**
 * A click anywhere during a keyboard drag ends it where the item stands. The
 * library does that on the window, after the click: a click into another
 * item of the same list reaches that item first, which then drops the drag
 * unannounced, picks itself up and takes the focus from the field clicked.
 * Ending the drag before the click arrives leaves that item nothing to do.
 */
function dropOnClick() {
	window.dispatchEvent(new MouseEvent('click'));
}

/**
 * A zone's `consider` and `finalize` handler: hands the list it should show
 * to `set`. `current` is the list as it stands before this event.
 *
 * A pointer drag passes straight through. A keyboard drag adds two things
 * the library lacks. Escape cancels: every list the drag touched (an item
 * can be tabbed into another ingredient group) goes back to the order it had
 * before. And the focus returns to the item's handle, where it now stands or,
 * after Escape, where it started: the library blurs the item when the drag
 * ends, which left the focus on the page and sent the next Tab to its top.
 * Only then, though - a click elsewhere also ends the drag (see
 * `dropOnClick`), and the focus stays where that click put it.
 */
export function applyDrag<T extends { id: unknown }>(
	event: CustomEvent<DndEvent<T>>,
	current: T[],
	set: (items: T[]) => void
): void {
	const { id, source, trigger } = event.detail.info;
	const list = event.currentTarget as Element;
	const flash = () => {
		const index = event.detail.items.findIndex((item) => item.id === id);
		if (index !== -1) void tick().then(() => flashPlaced(list.children[index]));
	};
	if (source !== SOURCES.KEYBOARD) {
		set(event.detail.items);
		// A pointer drag's finalize: dropped in a list, or back home from outside any.
		if (trigger === TRIGGERS.DROPPED_INTO_ZONE || trigger === TRIGGERS.DROPPED_OUTSIDE_OF_ANY)
			flash();
		return;
	}
	if (trigger === TRIGGERS.DRAG_STARTED) {
		document.addEventListener('keydown', noteEscape, true);
		document.addEventListener('click', dropOnClick, true);
		const index = current.findIndex((item) => item.id === id);
		keyboardDrag = { restore: new Map(), origin: { list, index }, escaped: false };
	}
	if (keyboardDrag && !keyboardDrag.restore.has(list)) {
		const before = [...current];
		keyboardDrag.restore.set(list, () => set(before));
	}
	if (trigger !== TRIGGERS.DRAG_STOPPED || !keyboardDrag) return set(event.detail.items);

	const { restore, origin, escaped } = keyboardDrag;
	keyboardDrag = null;
	document.removeEventListener('keydown', noteEscape, true);
	document.removeEventListener('click', dropOnClick, true);
	const home = escaped
		? origin
		: { list, index: event.detail.items.findIndex((item) => item.id === id) };
	if (escaped) restore.forEach((put) => put());
	else {
		set(event.detail.items);
		flash();
	}
	const active = document.activeElement;
	if (active !== null && active !== document.body) return;
	void tick().then(() =>
		home.list.children[home.index]?.querySelector<HTMLElement>('[role="button"]')?.focus()
	);
}
