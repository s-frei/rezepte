<script lang="ts">
	import type { Snippet } from 'svelte';
	import ViewSwitch from '#lib/components/shopping/ViewSwitch.svelte';
	import { m } from '#lib/paraglide/messages.js';

	/** The list page's panel, also the draft's: counts and view switch above the rows, or the empty note. */
	let {
		open,
		done,
		empty = false,
		children
	}: { open: number; done: number; empty?: boolean; children?: Snippet } = $props();
</script>

<div class="mt-4 rounded-2xl bg-surface shadow-card">
	<div
		class="mx-4.5 border-b border-dashed border-border py-3.5 md:mx-6 md:flex md:items-center md:justify-between md:py-3"
	>
		<p class="text-caption text-text-muted max-md:hidden">
			{m.list_counts({ open, done })}
		</p>
		<ViewSwitch compact />
	</div>
	<div class="px-4.5 pb-3 md:px-6 md:pt-1.5 md:pb-4">
		{#if empty}
			<div class="py-10 text-center">
				<p class="font-display text-heading font-medium">{m.list_empty_title()}</p>
				<p class="mx-auto mt-1.5 max-w-sm text-body text-text-muted">{m.list_empty_text()}</p>
			</div>
		{:else}
			{@render children?.()}
		{/if}
	</div>
</div>
