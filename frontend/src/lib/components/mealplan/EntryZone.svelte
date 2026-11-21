<script lang="ts">
	import type { Snippet } from 'svelte';
	import { flip } from 'svelte/animate';
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
		class: className = '',
		onconsider,
		onfinalize,
		onmove,
		onservings,
		onremove,
		empty
	}: {
		items: PlanEntry[];
		layout: 'column' | 'row' | 'chip';
		/** The zone's name for the drag announcements: the day, or the tray. */
		label: string;
		class?: string;
		onconsider: (event: CustomEvent<DndEvent<PlanEntry>>) => void;
		onfinalize: (event: CustomEvent<DndEvent<PlanEntry>>) => void;
		onmove: (entry: PlanEntry) => void;
		onservings: (entry: PlanEntry, servings: number) => void;
		onremove: (entry: PlanEntry) => void;
		empty?: Snippet;
	} = $props();

	const FLIP = $derived(prefersReducedMotion.current ? 0 : 150);

	/** The slot a dragged card would land in: as tall as the card, read when the drag starts. */
	let slot = $state(0);
	function consider(event: CustomEvent<DndEvent<PlanEntry>>) {
		const dragged = document.getElementById(DRAGGED_ELEMENT_ID);
		if (dragged) slot = dragged.getBoundingClientRect().height;
		onconsider(event);
	}

	// A finger held on a card: the zone's drag, not the browser's link menu.
	let touched = false;
	const note = (event: PointerEvent) => (touched = event.pointerType !== 'mouse');
	const guard = (event: Event) => {
		if (touched) event.preventDefault();
	};
</script>

<div class="relative min-h-11 {className}">
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
		onpointerdown={note}
		oncontextmenu={guard}
		class="min-h-11 {layout === 'column'
			? '-mb-[60vh] flex flex-col gap-2 pb-[60vh]'
			: layout === 'chip'
				? 'flex flex-col gap-1 md:flex-row md:flex-wrap md:gap-2'
				: 'flex flex-col gap-1.5 md:grid md:grid-cols-2 md:gap-x-5 md:[&>li:last-child:nth-child(odd)]:col-span-2'}"
	>
		{#each items as entry (entry.id)}
			<!-- Picked up, the copy that follows the pointer is a lifted card. -->
			<li
				data-entry-id={entry.id}
				animate:flip={{ duration: FLIP }}
				class="min-w-0 list-none outline-none [--flash-offset:2px] data-[lift=up]:*:rounded-xl data-[lift=up]:*:bg-surface data-[lift=up]:*:shadow-lift data-[lift=up]:*:outline-2 data-[lift=up]:*:outline-primary"
			>
				{#if isShadow(entry)}
					<div
						aria-hidden="true"
						style:height="{slot}px"
						class="rounded-xl border-2 border-dashed border-primary/60 bg-accent/40"
					></div>
				{:else}
					<PlanEntryCard
						{entry}
						{layout}
						onmove={() => onmove(entry)}
						onservings={(servings) => onservings(entry, servings)}
						onremove={() => onremove(entry)}
					/>
				{/if}
			</li>
		{/each}
	</ul>
	{#if items.length === 0 && empty}
		<div class="@container absolute inset-0">{@render empty()}</div>
	{/if}
</div>
