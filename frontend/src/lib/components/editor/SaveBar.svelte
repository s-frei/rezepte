<script lang="ts">
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import { backOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { scale } from 'svelte/transition';
	import Button from '#lib/components/ui/Button.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let {
		dirty = false,
		saving = false,
		saved = false,
		oncancel,
		onsave
	}: {
		/** Shows the "unsaved changes" marker. */
		dirty?: boolean;
		/** Disables both buttons and switches the primary label while a save runs. */
		saving?: boolean;
		/** The save went through: the button turns success-green, its check draws itself and it says so. */
		saved?: boolean;
		oncancel: () => void;
		onsave: () => void;
	} = $props();

	const DOT_DURATION = $derived(prefersReducedMotion.current ? 0 : 160);
</script>

<!--
	The label that shows and, in the same grid cell, the other one as an
	invisible stand-in, so the button is always as wide as the wider of the
	two and the swap moves nothing. The stand-in's words are generated
	content (`data-label`), not text: as text they were part of the
	button's `textContent`, which read "Save Saving…". "Saved" needs no
	stand-in: it shows for a moment before the page moves on, in a button
	that fills the rail anyway.
-->
{#snippet label()}
	<span class="grid justify-items-center">
		{#if saved}
			<span class="col-start-1 row-start-1 inline-flex items-center gap-2">
				<!-- Lucide's check with its path turned around, so it draws from
				     the short stroke to the long one, the way `Checkbox` ticks. -->
				<svg
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2.8"
					stroke-linecap="round"
					stroke-linejoin="round"
					aria-hidden="true"
					class="size-4"
				>
					<path
						d="m4 12 5 5L20 6"
						pathLength="1"
						class="[stroke-dasharray:1] motion-safe:animate-saved-draw"
					/>
				</svg>
				{m.common_saved()}
			</span>
		{:else if saving}
			<span class="col-start-1 row-start-1 inline-flex items-center gap-1.5">
				<LoaderCircle class="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
				{m.common_saving()}
			</span>
			<span
				aria-hidden="true"
				data-label={m.common_save()}
				class="invisible col-start-1 row-start-1 after:content-[attr(data-label)]"
			></span>
		{:else}
			<span class="col-start-1 row-start-1">{m.common_save()}</span>
			<span
				aria-hidden="true"
				data-label={m.common_saving()}
				class="invisible col-start-1 row-start-1 inline-flex items-center gap-1.5 after:content-[attr(data-label)]"
			>
				<span class="size-4"></span>
			</span>
		{/if}
	</span>
{/snippet}

<!--
	Desktop only; a phone has `EditorBar` instead. The foot of the section
	rail, under the nav entries, inside the rail's sticky block - so it
	scrolls with the rail and never lies over the form. Any bar that floats above a scrolling form hides a band of
	exactly what is being read; the rail is 200px wide and empty for the
	form's whole height, which is the only place on this page that can hold a
	control without taking anything away.

	Both buttons stack: 200px cannot hold them side by side, and a pill that
	breaks into two lines is worse than a column. "Speichern" goes on top,
	where the eye leaves the nav.

	The status line sits under the buttons, not above them, so its appearing
	never pushes them down under the pointer. A rule is all the separation
	the block needs - a card here would put a second surface in a rail whose
	entries sit straight on the page.
-->
<div
	class="hidden w-full flex-col gap-2 border-t border-border pt-4 md:flex"
	data-testid="save-actions"
>
	<Button
		variant="primary"
		disabled={saving}
		onclick={onsave}
		class="w-full {saved
			? 'bg-success-foreground! transition-none! motion-safe:animate-saved-ring'
			: ''}"
	>
		<!-- Saved switches the fill at once, with no `transition`: amber
		     tweened into green passes through olive, which reads as dirt.
		     The drawing check and the ring are the motion. -->
		{@render label()}
	</Button>
	<Button variant="ghost" disabled={saving || saved} onclick={oncancel} class="w-full">
		{m.common_cancel()}
	</Button>
	{#if dirty}
		<span class="mt-1 flex items-center justify-center gap-2 text-caption text-text-muted">
			<span
				in:scale={{ duration: DOT_DURATION, easing: backOut }}
				aria-hidden="true"
				class="size-2 shrink-0 rounded-pill bg-current"
			></span>
			<span>{m.editor_unsaved_changes()}</span>
		</span>
	{/if}
</div>
