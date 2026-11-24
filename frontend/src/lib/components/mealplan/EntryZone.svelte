<script lang="ts" module>
	import { on } from 'svelte/events';
	import { DRAGGED_ELEMENT_ID, SOURCES, type DndEvent } from 'svelte-dnd-action';

	// iOS clicks the link under the finger when it lifts from a long press
	// (with the callout off, the press ends as a tap), wherever the drag took
	// it: a trusted click on the dropped card or the lifted copy. So from a
	// pointer drag's start until shortly after its drop, the window swallows
	// every click, before the link or the router hear it. One for the whole
	// week: the drag runs across zones. Only a pointer's: a keyboard drop
	// sends no finalize. And while no lifted copy is left, a second at most.
	let swallowing: (() => void) | undefined;
	let settle: ReturnType<typeof setTimeout> | undefined;
	function unswallow() {
		swallowing?.();
		swallowing = undefined;
	}
	function backstop() {
		if (document.getElementById(DRAGGED_ELEMENT_ID)) settle = setTimeout(backstop, 1000);
		else unswallow();
	}
	function swallowClicks(event: CustomEvent<DndEvent<unknown>>) {
		if (event.detail.info.source !== SOURCES.POINTER) return dropped();
		clearTimeout(settle);
		settle = setTimeout(backstop, 1000);
		swallowing ??= on(
			window,
			'click',
			(event) => {
				event.preventDefault();
				event.stopPropagation();
			},
			{ capture: true }
		);
	}
	function dropped() {
		clearTimeout(settle);
		settle = setTimeout(unswallow, 400);
	}

	// Where the pointer is, for keeping it on the same spot of a copy that
	// changes its size (`asColumn`). One listener for the page, while any
	// zone is mounted: a drag starts from a press on a card.
	const pointer = { x: 0, y: 0 };
	let watching = 0;
	let unwatch: (() => void) | undefined;
	function watchPointer() {
		if (watching++ === 0) {
			const track = (event: PointerEvent) => {
				pointer.x = event.clientX;
				pointer.y = event.clientY;
			};
			const stops = [
				on(window, 'pointerdown', track, { capture: true, passive: true }),
				on(window, 'pointermove', track, { capture: true, passive: true })
			];
			unwatch = () => stops.forEach((stop) => stop());
		}
		return () => {
			if (--watching === 0) unwatch?.();
		};
	}

	/**
	 * A tray chip lifted while the week shows columns: its copy turns into
	 * the column card it would be on a day (`PlanEntryCard`'s drag face), a
	 * column wide, and moves so the pointer keeps its spot on it. Once per
	 * drag: the library calls this on every zone change.
	 */
	function asColumn(element: HTMLElement) {
		if (element.dataset.face || !element.querySelector('[data-drag-face]')) return;
		const column = document.querySelector<HTMLElement>('ul[data-layout="column"]');
		if (!column) return;
		element.dataset.face = 'column';
		const before = element.getBoundingClientRect();
		const fx = before.width ? (pointer.x - before.left) / before.width : 0.5;
		const fy = before.height ? (pointer.y - before.top) / before.height : 0.5;
		element.style.width = `${column.clientWidth}px`;
		element.style.height = 'auto';
		const after = element.getBoundingClientRect();
		const style = getComputedStyle(element);
		element.style.left = `${parseFloat(style.left) + pointer.x - (after.left + fx * after.width)}px`;
		element.style.top = `${parseFloat(style.top) + pointer.y - (after.top + fy * after.height)}px`;
	}
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import { flip } from 'svelte/animate';
	import { prefersReducedMotion } from 'svelte/motion';
	import { dndzone } from 'svelte-dnd-action';
	import type { PlanEntry } from '#lib/api/mealplan.js';
	import { isShadow, liftDragged, zoneOptions } from '#lib/recipe/dnd.js';
	import PlanEntryCard from './PlanEntryCard.svelte';

	/**
	 * A day's or the tray's entries as one drop zone of the week: every zone
	 * shares the type, so a card goes from any of them to any other. A mouse
	 * drags at once; a finger holds 300ms first (`delayTouchStart`), so a
	 * swipe still scrolls. The cards are no Tab stops of their own: the
	 * keyboard and a screen reader move an entry by its ⋯ ("Move to a
	 * day…"), and its link and ⋯ stay the stops. While empty the zone keeps
	 * its height and shows `empty` (the dashed "+ Plan a meal") over it, so
	 * a card can still be dropped there.
	 */
	let {
		items,
		layout,
		label,
		today,
		class: className = '',
		onconsider,
		onfinalize,
		onmove,
		onservings,
		onremove,
		empty,
		dragging = false
	}: {
		items: PlanEntry[];
		layout: 'column' | 'row' | 'chip';
		/** The zone's name for the drag announcements: the day, or the tray. */
		label: string;
		/** `YYYY-MM-DD`: the page's today, for the cards. */
		today: string;
		class?: string;
		onconsider: (event: CustomEvent<DndEvent<PlanEntry>>) => void;
		onfinalize: (event: CustomEvent<DndEvent<PlanEntry>>) => void;
		onmove: (entry: PlanEntry) => void;
		onservings: (entry: PlanEntry, servings: number) => void;
		onremove: (entry: PlanEntry) => void;
		empty?: Snippet;
		/** A card is being dragged somewhere in the week. */
		dragging?: boolean;
	} = $props();

	// While a card is dragged, a phone's day row takes a drop on its name
	// too: empty, the zone covers the whole row (its nearest positioned box),
	// the "+" staying where it was under it; once the slot is in, the zone
	// reaches up over the day's head (`-mt`, `pt` the head's height), so the
	// finger that brought the slot in stays over the zone and it does not
	// flicker out again.
	const cover = $derived(dragging && layout === 'row' && items.length === 0);
	const reach = $derived(dragging && layout === 'row' ? '-mt-12.5 pt-12.5' : '');

	const FLIP = $derived(prefersReducedMotion.current ? 0 : 150);

	/**
	 * The slot a dragged card would land in, as tall as what lands: on a day
	 * the lifted copy (a column card from the tray too, `asColumn`), in the
	 * tray its own first card, else a chip's height (a note's is lower). The
	 * empty tray takes its full shape for it (`WeekView`), so the slot
	 * previews the chip the drop makes.
	 */
	let slot = $state(0);
	let list = $state<HTMLElement>();
	function consider(event: CustomEvent<DndEvent<PlanEntry>>) {
		// The library holds the zone a drag leaves at its height until the
		// drop; the tray lets go, so a card taken out leaves no gap and the
		// last one leaves the calm pill, not a tall empty box.
		if (layout === 'chip' && list) list.style.minHeight = list.style.minWidth = '';
		const dragged = document.getElementById(DRAGGED_ELEMENT_ID);
		const own = list?.querySelector<HTMLElement>(
			':scope > li:not([data-is-dnd-shadow-item-internal])'
		);
		if (layout !== 'chip') {
			if (dragged) slot = dragged.getBoundingClientRect().height;
		} else if (own && own.id !== DRAGGED_ELEMENT_ID) {
			slot = own.getBoundingClientRect().height;
		} else {
			const shadow = event.detail.items.find(isShadow);
			slot = shadow && shadow.recipeId === null && shadow.servings === null ? 40 : 80;
		}
		swallowClicks(event);
		onconsider(event);
	}
	function finalize(event: CustomEvent<DndEvent<PlanEntry>>) {
		dropped();
		onfinalize(event);
	}

	// A finger held on a card: the zone's drag, not the browser's link menu
	// (Android's `contextmenu`, iOS's callout). By then the drag has lifted a
	// copy of the card into <body>, outside this list, and the long press
	// lands on the copy's link: so the window blocks the menu and the root
	// switches the callout off, which the copy inherits, until the last finger
	// lifts. A tap still opens the recipe; only the long press changes.
	// Captured on the list itself: the library stops the press at the card,
	// so Svelte's delegated handler would never hear it. A card taken off the
	// page mid-press sends its `touchend` nowhere, so the next first finger,
	// or the list going, lets go as well.
	let release: (() => void) | undefined;
	function hold() {
		if (release) return;
		const root = document.documentElement.style;
		root.setProperty('-webkit-touch-callout', 'none');
		const block = (event: Event) => event.preventDefault();
		const lifted = (event: TouchEvent) => {
			if (event.touches.length === 0) release?.();
		};
		const fresh = (event: TouchEvent) => {
			if (event.touches.length === 1) release?.();
		};
		const off = [
			on(window, 'contextmenu', block, { capture: true }),
			on(window, 'touchend', lifted, { capture: true }),
			on(window, 'touchcancel', lifted, { capture: true }),
			on(window, 'touchstart', fresh, { capture: true, passive: true })
		];
		release = () => {
			release = undefined;
			root.removeProperty('-webkit-touch-callout');
			for (const stop of off) stop();
		};
	}
	function guard(zone: HTMLElement) {
		const stop = on(zone, 'touchstart', hold, { capture: true, passive: true });
		return () => {
			stop();
			release?.();
		};
	}
</script>

<div
	class="min-h-11 {cover ? '' : 'relative'} {layout === 'column'
		? 'flex flex-1 flex-col'
		: ''} {className}"
>
	<ul
		aria-label={label}
		use:dndzone={{
			...zoneOptions(items, 'plan-entry', FLIP),
			transformDraggedElement: (element?: HTMLElement) => {
				liftDragged(element);
				if (element) asColumn(element);
			},
			delayTouchStart: 300,
			zoneItemTabIndex: -1,
			// No keyboard drag here, so none of the library's "press space"
			// hints or announcements: the ⋯ names the way ("Move to a day…").
			autoAriaDisabled: true,
			useCursorForDetection: true
		}}
		onconsider={consider}
		onfinalize={finalize}
		{@attach guard}
		{@attach watchPointer}
		bind:this={list}
		data-layout={layout}
		class="min-h-11 {cover ? 'absolute inset-0 z-10' : reach} {layout === 'column'
			? 'flex flex-1 flex-col gap-2'
			: layout === 'chip'
				? 'flex flex-col gap-1 md:grid md:grid-cols-2 md:gap-2 lg:grid-cols-[repeat(auto-fit,minmax(16rem,1fr))] md:max-lg:[&>li:last-child:nth-child(odd)]:col-span-2'
				: 'flex flex-col gap-1.5'}"
	>
		{#each items as entry (entry.id)}
			<!-- Picked up, the copy that follows the pointer is a lifted card: a
			     layer behind it, reaching past its edges, so the outline keeps
			     room to the photo and the stamps (the card itself clips its photo). -->
			<li
				data-entry-id={entry.id}
				animate:flip={{ duration: FLIP }}
				class="relative isolate min-w-0 list-none outline-none [--flash-offset:2px] before:pointer-events-none before:absolute before:-inset-2 before:-z-10 before:rounded-2xl before:bg-surface before:opacity-0 before:shadow-lift before:outline-2 before:outline-primary data-[lift=up]:*:shadow-none data-[lift=up]:before:opacity-100 dark:before:bg-surface-elevated dark:before:shadow-float-dark dark:data-[lift=up]:*:bg-surface-elevated"
			>
				{#if isShadow(entry)}
					<div
						aria-hidden="true"
						style:height="{slot}px"
						class="visible rounded-xl border-2 border-dashed border-primary/60 bg-accent/40"
					></div>
				{:else}
					<PlanEntryCard
						{entry}
						{layout}
						{today}
						onmove={() => onmove(entry)}
						onservings={(servings) => onservings(entry, servings)}
						onremove={() => onremove(entry)}
					/>
				{/if}
			</li>
		{/each}
	</ul>
	{#if items.length === 0 && empty}
		<div class="@container {cover ? 'h-11' : 'absolute inset-0'}">{@render empty()}</div>
	{/if}
</div>
