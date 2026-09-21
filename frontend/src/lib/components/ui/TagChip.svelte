<script lang="ts">
	import type { Snippet } from 'svelte';
	import X from 'lucide-svelte/icons/x';
	import { m } from '$lib/paraglide/messages';

	let {
		label,
		active = false,
		count,
		onclick,
		removable = false,
		onremove,
		leading
	}: {
		label: string;
		active?: boolean;
		count?: number;
		onclick?: (event: MouseEvent) => void;
		removable?: boolean;
		onremove?: () => void;
		/** Rendered before the label on a filter chip - the author filter
		 * puts the same initial there that its recipes carry on their cards,
		 * so the two read as one thing. */
		leading?: Snippet;
	} = $props();

	// A chip with a click handler is a filter chip (overview tag list); one
	// without is a plain display chip (recipe tags, editor tag list).
	const interactive = $derived(onclick !== undefined);
</script>

{#if interactive}
	<button
		type="button"
		{onclick}
		aria-pressed={active}
		class="inline-flex h-9 items-center gap-1.5 rounded-pill px-3.5 text-body-sm font-medium transition {active
			? 'bg-inverse text-inverse-foreground'
			: 'border border-border bg-surface text-text'}"
	>
		{#if leading}
			{@render leading()}
		{/if}
		{label}
		{#if count !== undefined}
			<span class={active ? 'text-inverse-muted' : 'text-text-muted'}>{count}</span>
		{/if}
	</button>
{:else}
	<span
		class="inline-flex items-center gap-1.5 rounded-pill bg-accent px-3 py-1 text-caption font-semibold text-accent-foreground"
	>
		{label}
		{#if count !== undefined}
			<span class="opacity-70">{count}</span>
		{/if}
		{#if removable}
			<button
				type="button"
				onclick={onremove}
				aria-label={m.common_remove()}
				class="opacity-60 transition hover:opacity-100"
			>
				<X class="size-3.5" />
			</button>
		{/if}
	</span>
{/if}
