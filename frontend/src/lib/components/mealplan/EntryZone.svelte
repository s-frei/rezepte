<script lang="ts">
	import type { Snippet } from 'svelte';
	import { flip } from 'svelte/animate';
	import { on } from 'svelte/events';
	import { prefersReducedMotion } from 'svelte/motion';
	import { DRAGGED_ELEMENT_ID, dndzone, type DndEvent } from 'svelte-dnd-action';
	import type { PlanEntry } from '#lib/api/mealplan.js';
	import { isShadow, zoneOptions } from '#lib/recipe/dnd.js';
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

	// While a card is dragged, a day row takes a drop on its name too: empty,
	// the zone covers the whole row (its nearest positioned box), the "+"
	// staying where it was under it; once the slot is in, the zone reaches
	// over the day's head - up on a phone (`-mt`, `pt` the head's height),
	// left over the name's column from `md` (`-ml`, `pl` the 152px column and
	// its gap, `DayRow`) - so the finger that brought the slot in stays over
	// the zone and it does not flicker out again.
	const cover = $derived(dragging && layout === 'row' && items.length === 0);
	const reach = $derived(
		dragging && layout === 'row' ? 'max-md:-mt-12.5 max-md:pt-12.5 md:-ml-42 md:pl-42' : ''
	);

	const FLIP = $derived(prefersReducedMotion.current ? 0 : 150);

	/** The slot a dragged card would land in: as tall as the card, read when the drag starts. */
	let slot = $state(0);
	function consider(event: CustomEvent<DndEvent<PlanEntry>>) {
		const dragged = document.getElementById(DRAGGED_ELEMENT_ID);
		if (dragged) slot = dragged.getBoundingClientRect().height;
		onconsider(event);
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

<div class="min-h-11 {cover ? '' : 'relative'} {className}">
	<ul
		aria-label={label}
		use:dndzone={{
			...zoneOptions(items, 'plan-entry', FLIP),
			delayTouchStart: 300,
			zoneItemTabIndex: -1,
			// No keyboard drag here, so none of the library's "press space"
			// hints or announcements: the ⋯ names the way ("Move to a day…").
			autoAriaDisabled: true,
			useCursorForDetection: true
		}}
		onconsider={consider}
		{onfinalize}
		{@attach guard}
		class="min-h-11 {cover ? 'absolute inset-0 z-10' : reach} {layout === 'column'
			? '-mb-[60vh] flex flex-col gap-2 pb-[60vh]'
			: layout === 'chip'
				? 'flex flex-col gap-1 md:grid md:grid-cols-2 md:gap-2 lg:grid-cols-[repeat(auto-fit,minmax(16rem,1fr))] md:max-lg:[&>li:last-child:nth-child(odd)]:col-span-2'
				: 'flex flex-col gap-1.5 md:grid md:grid-cols-2 md:gap-x-5 md:[&>li:last-child:nth-child(odd)]:col-span-2'}"
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
