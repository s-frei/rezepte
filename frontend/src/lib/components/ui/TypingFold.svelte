<script lang="ts">
	import type { Snippet } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { pinWhileTyping } from '#lib/visible-area.js';

	/**
	 * The top of a phone's search sheet (title, text, chips), folded away
	 * while `shut` - its field has focus and a soft keyboard is up - so the
	 * field heads the sheet and the results fill the room above the keyboard.
	 * Folding, the sheet's scroll goes to its top, so `field` rises as the
	 * parts above it shrink; a pin meanwhile scrolled it out of view as they
	 * went. Folded, `field` is pinned whenever the keyboard resizes the view
	 * (`pinWhileTyping`). A keyboard put away without a blur (Android's back)
	 * gives the room back: the focus moves from the field to the sheet, so
	 * the top comes back and a screen reader stays in the dialog.
	 * Its padding, canceled by margins, keeps focus rings clear of the clip.
	 * `held` is on from the fold until the unfold is done: the sheet keeps
	 * its full height (`fill`) that long, so it settles to its content's
	 * height once that is known instead of dropping and growing back.
	 */
	let {
		shut,
		field,
		// Only written here, for the parent's `bind:held`.
		// eslint-disable-next-line no-useless-assignment
		held = $bindable(false),
		children
	}: { shut: boolean; field?: HTMLElement; held?: boolean; children: Snippet } = $props();

	$effect(() => {
		if (shut) held = true;
		else if (prefersReducedMotion.current) held = false;
	});

	/** The fold is done; a new run of the transition starts over. */
	let folded = $state(false);

	$effect(() => {
		if (!shut || !field) return;
		if (folded || prefersReducedMotion.current) return pinWhileTyping(field);
		field.closest('[data-sheet-body]')?.scrollTo({ top: 0 });
	});

	$effect(() => {
		const viewport = window.visualViewport;
		if (!shut || !viewport) return;
		let lowest = viewport.height;
		const resize = () => {
			lowest = Math.min(lowest, viewport.height);
			// More than a toolbar: the keyboard went.
			if (viewport.height > lowest + 120)
				field?.closest<HTMLElement>('[role="dialog"]')?.focus({ preventScroll: true });
		};
		viewport.addEventListener('resize', resize);
		return () => viewport.removeEventListener('resize', resize);
	});

	const rows = (event: TransitionEvent) =>
		event.target === event.currentTarget && event.propertyName === 'grid-template-rows';
</script>

<div
	ontransitionrun={(event) => {
		if (rows(event)) folded = false;
	}}
	ontransitionend={(event) => {
		if (!rows(event)) return;
		folded = shut;
		if (!shut) held = false;
	}}
	class="grid transition-[grid-template-rows,opacity] duration-200 ease-out motion-reduce:transition-none {shut
		? 'grid-rows-[0fr] opacity-0'
		: 'grid-rows-[1fr]'}"
>
	<div class="-m-1 min-h-0 overflow-hidden p-1">{@render children()}</div>
</div>
