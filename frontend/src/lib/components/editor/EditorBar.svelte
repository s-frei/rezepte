<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import X from '@lucide/svelte/icons/x';
	import { backOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade, scale } from 'svelte/transition';
	import type { ContentsEntry } from '#lib/components/nav/contents.js';
	import { m } from '#lib/paraglide/messages.js';

	let {
		entries,
		current,
		progress,
		dirty,
		saving,
		saved = false,
		errorIds,
		expanded,
		onopen,
		oncancel,
		onsave
	}: {
		entries: ContentsEntry[];
		/** Id of the entry the bar names. */
		current: string;
		/** How far the reader is through the current entry, 0 to 1. */
		progress: number;
		/** Shows the "unsaved changes" marks. */
		dirty: boolean;
		/** Disables both buttons and spins the check while a save runs. */
		saving: boolean;
		/** The save went through: the check turns success-green and draws itself. */
		saved?: boolean;
		/** Ids of the entries holding an error: their segments turn destructive. */
		errorIds: Set<string>;
		/** Whether the contents sheet the middle opens is open. */
		expanded: boolean;
		onopen: () => void;
		oncancel: () => void;
		onsave: () => void;
	} = $props();

	const index = $derived(
		Math.max(
			0,
			entries.findIndex((entry) => entry.id === current)
		)
	);
	const label = $derived(entries[index]?.label ?? '');
	const duration = $derived(prefersReducedMotion.current ? 0 : 160);

	/** Fill of segment `i`: whole before the current one, partial on it, none after. */
	function fill(i: number): number {
		if (i < index) {
			return 1;
		}
		if (i > index) {
			return 0;
		}
		return Math.min(1, Math.max(0, progress));
	}
</script>

<!--
	The editor's phone header, one 60px row pinned to the top: Cancel, the
	section being read, Save. It is pinned to the top, not the bottom: the
	software keyboard owns the bottom while the cook types a step. The middle
	is the running head of a cookbook - the section's name over one segment
	per section that fills with reading progress - and opens the contents
	sheet; a section holding an error turns its segment destructive. The
	recipe's title is not here: it is being typed in the form below.

	Unsaved changes show twice: a dot on the check, and "● edited" after the
	name. What does not fit drops out without measuring, wrapped onto a
	clipped second line: first the word "edited", then the position, and
	the name truncates once it fills the line alone. The dot after the name
	stays, so the bar says "unsaved" at any width and with any section name.
	The edited mark is accent-foreground in light, where primary misses
	4.5:1 on the background at this size, and primary in dark.

	While the bar shows, the page's `scroll-padding-top` is its height, so
	the browser scrolls a focused field out from under it rather than
	calling it in view (see the style below); the sections' `scroll-mt-4`
	is only the breath on top of that.

	Every clipping box here is `overflow-clip`, never `overflow-hidden` or
	`auto`: clip is not a scroll container, so no browser can give it a
	scrollbar for a pixel of descender or the wrapped second line.

	Opaque, not frosted: a form scrolling under a blur read as noise behind
	the section name.
-->
<div
	data-covers-top
	data-editor-bar
	class="fixed inset-x-0 top-0 z-40 flex h-15 items-center gap-2.5 border-b border-border bg-background px-3 md:hidden"
>
	<button
		type="button"
		disabled={saving || saved}
		onclick={oncancel}
		aria-label={m.common_cancel()}
		class="grid size-11 shrink-0 place-items-center rounded-full border-[1.5px] border-border bg-surface hover-tint text-text transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] disabled:opacity-45"
	>
		<X class="size-4.5" strokeWidth={2.4} aria-hidden="true" />
	</button>

	<button
		type="button"
		onclick={onopen}
		aria-haspopup="dialog"
		aria-expanded={expanded}
		aria-label={m.nav_contents_open({ current: label })}
		class="flex min-h-11 min-w-0 flex-1 flex-col justify-center gap-1.5 rounded-md px-0.5 py-1 text-left focus-visible:outline-2 focus-visible:outline-primary"
	>
		<span class="flex h-6 items-baseline gap-x-1.5 overflow-clip">
			<span class="flex min-w-0 flex-wrap items-baseline gap-x-1.5">
				<!-- One grid cell for the outgoing and the incoming name, so the two
				     cross-fade in place instead of stacking. -->
				<span class="grid min-w-0">
					{#key label}
						<span
							class="col-start-1 row-start-1 min-w-0 overflow-clip font-display text-card leading-6 font-medium text-ellipsis whitespace-nowrap"
							transition:fade={{ duration }}
						>
							{label}
						</span>
					{/key}
				</span>
				<span class="shrink-0 text-micro font-semibold text-text-muted tabular-nums">
					{m.editor_section_position({ index: index + 1, count: entries.length })}
				</span>
			</span>
			{#if dirty}
				<!-- Sized as the dot, and grown into whatever room the name and
				     the position leave, up to the word: the word wraps away
				     first, the dot never does. -->
				<span
					class="inline-flex max-w-max min-w-0 shrink-0 grow basis-1.75 flex-wrap items-center gap-x-1 text-micro font-semibold text-accent-foreground dark:text-primary"
				>
					<span aria-hidden="true" class="size-1.75 rounded-pill bg-current"></span>
					{m.editor_edited()}
				</span>
			{/if}
		</span>
		<span aria-hidden="true" class="flex gap-1">
			{#each entries as entry, i (entry.id)}
				{@const error = errorIds.has(entry.id)}
				<span
					class="h-0.75 flex-1 overflow-clip rounded-pill {error ? 'bg-destructive' : 'bg-border'}"
				>
					<span
						class="block h-full {error ? 'bg-destructive' : 'bg-primary'}"
						style:width="{fill(i) * 100}%"
					></span>
				</span>
			{/each}
		</span>
	</button>

	<button
		type="button"
		disabled={saving}
		onclick={onsave}
		aria-label={m.common_save()}
		class="relative grid size-11 shrink-0 place-items-center rounded-full text-primary-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] disabled:opacity-80 {saved
			? 'bg-success-foreground motion-safe:animate-saved-ring'
			: 'bg-primary transition'}"
	>
		<!-- Saved switches the fill at once, with no `transition`: amber
		     tweened into green passes through olive, which reads as dirt.
		     The drawing check and the ring are the motion. -->
		{#if saved}
			<!-- Lucide's check with its path turned around, so it draws from the
			     short stroke to the long one, the way `Checkbox` ticks. -->
			<svg
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2.8"
				stroke-linecap="round"
				stroke-linejoin="round"
				aria-hidden="true"
				class="size-5"
			>
				<path
					d="m4 12 5 5L20 6"
					pathLength="1"
					class="[stroke-dasharray:1] motion-safe:animate-saved-draw"
				/>
			</svg>
		{:else if saving}
			<LoaderCircle class="size-5 animate-spin motion-reduce:animate-none" aria-hidden="true" />
		{:else}
			<Check class="size-5" strokeWidth={2.8} aria-hidden="true" />
		{/if}
		{#if dirty}
			<span
				in:scale={{ duration, easing: backOut }}
				aria-hidden="true"
				class="absolute top-0 right-0 size-3 rounded-pill bg-primary ring-2 ring-background"
			></span>
		{/if}
	</button>
	{#if dirty}
		<span class="sr-only overflow-clip!">{m.editor_unsaved_changes()}</span>
	{/if}
</div>

<style>
	/* `:has`, not a plain `:global(html)`: a component's styles stay in the
	   document after it unmounts, and the rest of the app has no bar to clear.
	   48rem is `md`, where the bar hides. */
	@media (width < 48rem) {
		:global(html:has([data-editor-bar])) {
			scroll-padding-top: calc(var(--spacing) * 15);
		}
	}
</style>
