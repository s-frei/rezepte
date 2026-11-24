<script lang="ts">
	import Plus from '@lucide/svelte/icons/plus';
	import type { Snippet } from 'svelte';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * "Someday": the entries without a day. On a phone a card above the days;
	 * from `md` its head above two even columns of cards, from `lg` a compact
	 * strip over the whole width, the head beside its cards, which grow to
	 * fill it (no blank end).
	 * Empty, it is one calm dashed pill on one line, and still takes a drop. Its "+"
	 * plans straight onto it, as a day's "+" does for the day.
	 *
	 * From `md` its cards sit 6px in, so its radius is theirs plus 6
	 * (concentric, [Design system]); the "+" sits 10px in, its radius 14.
	 */
	let { empty, onadd, children }: { empty: boolean; onadd: () => void; children: Snippet } =
		$props();
</script>

<section
	aria-label={m.plan_tray()}
	data-tray
	tabindex="-1"
	class={empty
		? 'flex min-h-12.5 items-center gap-x-2 rounded-pill border-[1.5px] border-dashed border-border pl-4'
		: 'rounded-2xl bg-surface pt-2.5 pr-1.5 pb-1.5 pl-4 shadow-card min-[1200px]:gap-5 md:rounded-3xl md:pl-4.5 lg:flex lg:items-center lg:gap-3.5 lg:py-1.5'}
>
	{#snippet add(more: string)}
		<button
			type="button"
			aria-label={m.plan_tray_add()}
			data-tray-add
			aria-haspopup="dialog"
			onclick={onadd}
			class="inline-flex size-11 shrink-0 items-center justify-center rounded-full text-text-muted transition hover:text-text focus-visible:outline-2 focus-visible:outline-primary md:size-7 {more}"
		>
			<Plus class="size-4.5 md:size-4" aria-hidden="true" />
		</button>
	{/snippet}
	<!-- One zone element in both shapes: a drag flips the shape as a card
	     leaves or enters (`WeekView`), and a zone rebuilt mid-drag would
	     drop it. -->
	<div
		class={empty
			? 'flex items-baseline'
			: 'flex items-center gap-2 lg:shrink-0 lg:items-start lg:self-start lg:pt-1 lg:max-[1199.98px]:max-w-38'}
	>
		<div class="flex min-w-0 flex-1 flex-wrap items-baseline gap-x-2.5 lg:flex-col">
			<span class="font-display text-card-sm font-medium">{m.plan_tray()}</span>
			{#if !empty}<span class="text-caption text-text-muted">{m.plan_tray_hint()}</span>{/if}
		</div>
		{#if !empty}{@render add('-my-2.5 hover:bg-background md:my-0 md:mr-1 lg:mr-0')}{/if}
	</div>
	<div class="@container min-w-0 flex-1 {empty ? '' : 'mt-1 md:mt-2 lg:mt-0'}">
		{@render children()}
	</div>
	{#if empty}
		<!-- A pill 50px tall: the "+" as far in from the end as from the top
		     (3px, from md 11px, measured from the outer edge), so it is concentric. -->
		{@render add('mr-0.5 hover:bg-surface md:mr-2.5')}
	{/if}
</section>
