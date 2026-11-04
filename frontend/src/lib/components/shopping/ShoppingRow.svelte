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
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	let {
		row: given,
		onToggle,
		onOpen,
		bySource = false,
		shop = false,
		flash = false,
		checkers = false
	}: {
		row: Row;
		onToggle: (row: Row) => void;
		/** Opens the item sheet; the shopping mode has none. */
		onOpen?: (row: Row) => void;
		/** The recipe view: the block already names the recipe, so the pairs leave it out. */
		bySource?: boolean;
		/** The shopping mode's larger row: each pair on its own line, a tap anywhere ticks. */
		shop?: boolean;
		/** Just added: the row lights up once, where it is; the page does not scroll, so the next item is typed right away. */
		flash?: boolean;
		/** A shared list: a ticked row shows who ticked it. */
		checkers?: boolean;
	} = $props();

	// Ticked elsewhere a moment ago: shown ticked and lit before it moves to Done.
	const lingers = $derived(given.lingering);
	const row = $derived(lingers ? { ...given, checked: true } : given);

	const pairs = $derived(
		row.sources
			.map((s) => ({
				id: s.itemId,
				amount: s.amount,
				origin: sourceOrigin(s, bySource),
				checked: s.checked
			}))
			.filter((p) => p.amount || p.origin)
	);
	const notes = $derived([...new Set(row.sources.map((s) => s.note).filter(Boolean))]);
	/** What is still to buy, beside the name, once there is more than one amount; a done row needs none. */
	const total = $derived(pairs.length > 1 && !row.checked ? row.total : '');
	const done = $derived(row.checked ? 'text-text-muted line-through' : '');
	const tickers = $derived(checkers && row.checked ? row.checkedBy : []);
</script>

<!--
	Row rules (spec, "Approved look"): the name breaks only between words;
	every "amount · origin" pair takes its own line (owner's call, 2026-11-06),
	nowrap: only the origin may shrink to an ellipsis, the amount never. With
	more than one pair the amounts added up per unit stand right of the name
	(`total`). The dot is bound to the origin by
	no-break spaces (`&nbsp;·&nbsp;`). The 22px circle sits on the name's 22.5px first line.
	In the shopping mode (`shop`) the 30px circle
	sits on the name's 24px first line, the circle's button stretches over the
	whole row, so a tap on the name ticks too, and every row keeps its rule.
	On the list page the circle's padding makes a 44px target, and the name's
	button stretches (`::after`, phones only) over the text column and the row's padding,
	so a tap on the amounts opens the sheet too; neither moves anything.
-->
<li
	class="flex items-start border-b border-dashed border-border py-2.5 {flash || lingers
		? '-mx-2 rounded-md px-2 motion-safe:animate-flash motion-reduce:bg-accent'
		: ''} {shop ? 'relative min-h-15 gap-4' : 'gap-3 last:border-b-0'}"
>
	<button
		type="button"
		aria-pressed={row.checked}
		aria-label={m.list_tick({ name: row.name })}
		onclick={() => onToggle(row)}
		class="shrink-0 rounded-full focus-visible:outline-2 focus-visible:outline-primary {shop
			? '-mt-[3px] after:absolute after:inset-0 focus-visible:outline-offset-2'
			: '-m-[11px] p-[11px] focus-visible:outline-offset-[-7px]'}"
	>
		<span
			class="flex items-center justify-center rounded-full transition {shop
				? 'size-[30px] border-2'
				: 'size-[22px] border-[1.5px]'} {row.checked
				? 'border-primary bg-primary text-primary-foreground'
				: 'border-text-muted bg-surface-elevated'}"
		>
			{#if row.checked}<Check
					class={shop ? 'size-4' : 'size-3.5'}
					strokeWidth={3}
					aria-hidden="true"
				/>{/if}
		</span>
	</button>
	<!-- Positioned only for the name's target: in the shopping mode it would cover the circle's. -->
	<div class="min-w-0 flex-1 {onOpen ? 'relative' : ''}">
		<div class="flex items-baseline justify-between gap-3">
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
					class="block max-w-full text-left text-body font-medium text-pretty wrap-normal after:absolute after:inset-x-0 after:-inset-y-[11px] hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:after:hidden {done}"
				>
					{row.name}
				</button>
			{/if}
			{#if total}
				<span
					class="shrink-0 font-bold {shop ? 'text-body' : 'text-body-sm'} {row.checked
						? 'text-text-muted line-through'
						: 'text-primary'}">{total}</span
				>
			{/if}
		</div>
		{#if pairs.length > 0 || notes.length > 0}
			<div
				class="flex flex-col items-start gap-y-0.5 text-text-muted {shop
					? 'mt-1 text-body-sm'
					: 'mt-0.5 text-caption'}"
			>
				{#each pairs as pair (pair.id)}
					<!-- A pair ticked by recipe while the row is open is struck through, and left out of the total. -->
					<span
						data-source
						class="inline-flex max-w-full min-w-0 whitespace-nowrap {pair.checked
							? 'line-through'
							: ''}"
					>
						{#if pair.amount}
							<span
								class="flex-none font-bold {row.checked || pair.checked
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
					<span class="italic">{note}</span>
				{/each}
			</div>
		{/if}
	</div>
	{#if tickers.length > 0}
		{@const label = m.list_checked_by({ names: tickers.map((p) => p.displayName).join(', ') })}
		<!-- One ticker: mark and first name; several: their marks only. -->
		<span
			role="img"
			aria-label={label}
			title={label}
			class="flex max-w-28 min-w-0 shrink-0 items-center gap-1.5 self-center text-caption text-text-muted"
		>
			<span class="flex shrink-0">
				{#each tickers as person, i (person.id)}
					<PersonMark
						{person}
						size="xs-plus"
						class="ring-2 ring-surface {i > 0 ? '-ml-1.5' : ''}"
					/>
				{/each}
			</span>
			{#if tickers.length === 1}
				<span aria-hidden="true" class="truncate"
					>{tickers[0].id === session.user?.id
						? m.list_ticker_you()
						: tickers[0].displayName.split(' ')[0]}</span
				>
			{/if}
		</span>
	{/if}
</li>
