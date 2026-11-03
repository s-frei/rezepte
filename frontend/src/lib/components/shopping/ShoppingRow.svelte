<script lang="ts" module>
	import { session } from '#lib/auth.svelte.js';
	import type { Row, Source } from '#lib/shopping/rows.js';
	import { m } from '#lib/paraglide/messages.js';

	/** What follows a source's amount: its recipe (unless the block names it), or who typed it in. */
	export function sourceOrigin(source: Source, bySource = false): string | null {
		if (source.recipe) return bySource ? null : source.recipe;
		if (source.formerMember) return m.list_added_by_former();
		if (source.addedById && source.addedById === session.user?.id) return m.list_added_by_you();
		return source.addedBy ? m.list_added_by({ name: source.addedBy }) : null;
	}
</script>

<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	let {
		row,
		onToggle,
		onOpen,
		bySource = false,
		shop = false
	}: {
		row: Row;
		onToggle: (row: Row) => void;
		/** Opens the item sheet; the shopping mode has none. */
		onOpen?: (row: Row) => void;
		/** The recipe view: the block already names the recipe, so the pairs leave it out. */
		bySource?: boolean;
		/** The shopping mode's larger row: each pair on its own line, a tap anywhere ticks. */
		shop?: boolean;
	} = $props();

	const pairs = $derived(
		row.sources
			.map((s) => ({ id: s.itemId, amount: s.amount, origin: sourceOrigin(s, bySource) }))
			.filter((p) => p.amount || p.origin)
	);
	const notes = $derived([...new Set(row.sources.map((s) => s.note).filter(Boolean))]);
	const done = $derived(row.checked ? 'text-text-muted line-through' : '');
</script>

<!--
	Row rules (spec, "Approved look"): the name breaks only between words;
	each "amount · origin" pair is one nowrap flex item, so a pair that does
	not fit moves to the next line whole; inside a pair only the origin may
	shrink to an ellipsis, the amount never. The dot is bound to the origin by
	no-break spaces (`&nbsp;·&nbsp;`). The 22px circle sits on the name's 22.5px first line.
	In the shopping mode (`shop`) every pair takes its own line, the 30px circle
	sits on the name's 24px first line, the circle's button stretches over the
	whole row, so a tap on the name ticks too, and every row keeps its rule.
-->
<li
	class="flex items-start border-b border-dashed border-border py-2.5 {shop
		? 'relative min-h-15 gap-4'
		: 'gap-3 last:border-b-0'}"
>
	<button
		type="button"
		aria-pressed={row.checked}
		aria-label={m.list_tick({ name: row.name })}
		onclick={() => onToggle(row)}
		class="shrink-0 rounded-full focus-visible:outline-2 focus-visible:outline-primary {shop
			? '-mt-[3px] after:absolute after:inset-0 focus-visible:outline-offset-2'
			: '-m-2.5 p-2.5 focus-visible:outline-offset-[-6px]'}"
	>
		<span
			class="flex items-center justify-center rounded-full transition {shop
				? 'size-[30px] border-2'
				: 'size-[22px] border-[1.5px]'} {row.checked
				? 'border-primary bg-primary text-primary-foreground'
				: 'border-handle bg-surface-elevated'}"
		>
			{#if row.checked}<Check
					class={shop ? 'size-4' : 'size-3.5'}
					strokeWidth={3}
					aria-hidden="true"
				/>{/if}
		</span>
	</button>
	<div class="min-w-0 flex-1">
		{#if !onOpen}
			<p
				class="text-pretty wrap-normal {shop
					? 'text-card leading-6 font-medium'
					: 'text-body font-medium'} {done}"
			>
				{row.name}
			</p>
		{:else}
			<button
				type="button"
				aria-haspopup="dialog"
				onclick={() => onOpen(row)}
				class="block max-w-full text-left text-body font-medium text-pretty wrap-normal hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {done}"
			>
				{row.name}
			</button>
		{/if}
		{#if pairs.length > 0 || notes.length > 0}
			<div
				class="text-text-muted {shop
					? 'mt-1 flex flex-col items-start gap-y-0.5 text-body-sm'
					: 'mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-caption'}"
			>
				{#each pairs as pair (pair.id)}
					<span data-source class="inline-flex max-w-full min-w-0 whitespace-nowrap">
						{#if pair.amount}
							<span
								class="flex-none font-bold {row.checked
									? 'text-text-muted line-through'
									: 'text-primary'}">{pair.amount}</span
							>
						{/if}
						{#if pair.origin}
							<span class="min-w-0 truncate"
								>{#if pair.amount}&nbsp;·&nbsp;{/if}{pair.origin}</span
							>
						{/if}
					</span>
				{/each}
				{#each notes as note (note)}
					<span class="basis-full italic">{note}</span>
				{/each}
			</div>
		{/if}
	</div>
</li>
