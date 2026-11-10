<script lang="ts">
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade } from 'svelte/transition';
	import { m } from '#lib/paraglide/messages.js';
	import type { ContentsEntry } from './contents';

	let {
		entries,
		current,
		expanded = false,
		onopen,
		class: className = ''
	}: {
		entries: ContentsEntry[];
		/** Id of the entry the head names. */
		current: string;
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
	const Icon = $derived(entries[index]?.icon);
	const duration = $derived(prefersReducedMotion.current ? 0 : 160);
</script>

<!--
	A cookbook's running head: the page says where you are in a single line,
	and the contents sheet it opens takes you anywhere else. It stays one line
	however many entries there are, which is what lets it carry any number of
	them. Full-bleed so what scrolls under it is covered edge to edge; the
	name swapping is its only motion. It heads pages (settings), where the
	name alone looked like a heading, so it is a select-like field - surface,
	border, the page's icon, the chevron at the far end - that reads as a
	control to change the page and not as a second title. The name wraps
	rather than ending in "…": with 200% text a page name like "Benutzer" no
	longer fits a phone's line. The editor's scroll has its own head with
	reading progress, `EditorBar`.
-->
<div
	data-covers-top
	class="sticky z-30 -mx-5 bg-background/90 px-5 py-2 backdrop-blur md:hidden {className}"
>
	<button
		type="button"
		onclick={onopen}
		aria-haspopup="dialog"
		aria-expanded={expanded}
		aria-label={m.nav_contents_open({ current: label })}
		class="flex min-h-12 w-full items-center gap-2.5 rounded-xl border border-border bg-surface/80 px-3.5 py-1.5 text-left focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
	>
		{#if Icon}
			<Icon class="size-4.5 shrink-0 text-text-muted" aria-hidden="true" />
		{/if}
		<!-- One grid cell for the outgoing and the incoming name, so the two
		     cross-fade in place instead of stacking. -->
		<span class="grid min-w-0">
			{#key label}
				<span
					class="col-start-1 row-start-1 font-display text-body-lg font-medium wrap-break-word hyphens-auto"
					transition:fade={{ duration }}
				>
					{label}
				</span>
			{/key}
		</span>
		<ChevronDown class="ml-auto size-4 shrink-0 text-text-muted" aria-hidden="true" />
	</button>
</div>
