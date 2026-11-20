<script lang="ts">
	import type { Snippet } from 'svelte';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * "Someday": the entries without a day. On a phone a card above the days,
	 * from `md` a compact strip over the whole width, its cards in a row.
	 * Empty, it is one calm dashed line, and still takes a drop.
	 */
	let { empty, children }: { empty: boolean; children: Snippet } = $props();
</script>

<section
	aria-label={m.plan_tray()}
	data-tray
	tabindex="-1"
	class="rounded-2xl {empty
		? 'flex flex-wrap items-center gap-x-2 border-[1.5px] border-dashed border-border py-0.5 pr-2 pl-4'
		: 'bg-surface pt-2.5 pr-1.5 pb-1.5 pl-4 shadow-card min-[1200px]:gap-5 md:flex md:items-center md:gap-3.5 md:py-2 md:pr-2 md:pl-4.5'}"
>
	<div
		class={empty
			? 'flex items-baseline'
			: 'flex flex-wrap items-baseline gap-x-2.5 md:shrink-0 md:flex-col md:max-[1199.98px]:max-w-38'}
	>
		<span class="font-display text-card-sm font-medium">{m.plan_tray()}</span>
		{#if !empty}
			<span class="text-caption text-text-muted">{m.plan_tray_hint()}</span>
		{/if}
	</div>
	<div class="min-w-0 flex-1 {empty ? '' : 'mt-1 md:mt-0'}">{@render children()}</div>
</section>
