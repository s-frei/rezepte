<script lang="ts">
	import ChevronDown from 'lucide-svelte/icons/chevron-down';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade } from 'svelte/transition';
	import { m } from '$lib/paraglide/messages';
	import type { ContentsEntry } from './contents';

	let {
		entries,
		current,
		progress,
		expanded = false,
		onopen,
		class: className = ''
	}: {
		entries: ContentsEntry[];
		/** Id of the entry the head names. */
		current: string;
		/**
		 * How far the reader is through the current entry, 0 to 1. Set, it
		 * draws the hairline of one segment per entry under the name; unset
		 * (pages rather than a scroll), there is no hairline.
		 */
		progress?: number;
		/** Whether the contents sheet this head opens is open. */
		expanded?: boolean;
		onopen: () => void;
		/** Where it sticks: the caller knows what is pinned above it. */
		class?: string;
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
		return Math.min(1, Math.max(0, progress ?? 0));
	}
</script>

<!--
	A cookbook's running head: the page says where you are in a single line,
	and the contents sheet it opens takes you anywhere else. It stays one line
	however many entries there are, which is what lets it carry any number of
	them. Full-bleed so what scrolls under it is covered edge to edge; the
	name swapping is its only motion.
-->
<div class="sticky z-30 -mx-5 bg-background/90 backdrop-blur md:hidden {className}">
	<button
		type="button"
		onclick={onopen}
		aria-haspopup="dialog"
		aria-expanded={expanded}
		aria-label={m.nav_contents_open({ current: label })}
		class="flex h-12 w-full items-center gap-1.5 px-5 text-left focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
	>
		<!-- One grid cell for the outgoing and the incoming name, so the two
		     cross-fade in place instead of stacking. -->
		<span class="grid min-w-0">
			{#key label}
				<span
					class="col-start-1 row-start-1 truncate font-display text-card font-medium"
					transition:fade={{ duration }}
				>
					{label}
				</span>
			{/key}
		</span>
		<ChevronDown class="size-4 shrink-0 translate-y-px text-text-muted" aria-hidden="true" />
	</button>
	{#if progress !== undefined}
		<!-- The name already says where the reader is; this only shows it. -->
		<div aria-hidden="true" class="flex gap-1 px-5">
			{#each entries as entry, i (entry.id)}
				<span class="h-0.5 flex-1 overflow-hidden rounded-pill bg-border">
					<span class="block h-full bg-primary" style:width="{fill(i) * 100}%"></span>
				</span>
			{/each}
		</div>
	{:else}
		<div aria-hidden="true" class="mx-5 h-px bg-border"></div>
	{/if}
</div>
