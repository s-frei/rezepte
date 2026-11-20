<script lang="ts">
	import type { Snippet } from 'svelte';
	import Plus from '@lucide/svelte/icons/plus';
	import { dayNames } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';

	/**
	 * A day as a card: on a phone the name and date over its entries, from
	 * `md` to the desktop's columns a row with the name, date and "+" in a
	 * label column beside the entries. An empty day is one line, its name
	 * and the dashed "+ Plan a meal". Today's card is outlined and wears a
	 * "Today" pill.
	 */
	let {
		day,
		today,
		empty,
		onadd,
		children
	}: {
		day: string;
		today: boolean;
		empty: boolean;
		onadd: () => void;
		children: Snippet;
	} = $props();

	const names = $derived(dayNames(day, getLocale()));
</script>

<section
	aria-label={names.full}
	aria-current={today ? 'date' : undefined}
	class="rounded-2xl bg-surface shadow-card {today ? 'outline-2 outline-primary' : ''} {empty
		? 'flex items-center gap-3 py-1.5 pr-3 pl-4 md:gap-4 md:py-2.5 md:pl-4.5'
		: 'pt-1.5 pr-3 pb-3 pl-4 md:grid md:grid-cols-[120px_minmax(0,1fr)] md:items-start md:gap-4 md:py-2.5 md:pl-4.5'}"
>
	<header
		class="flex min-h-11 items-center gap-2 {empty
			? 'flex-none md:min-w-30'
			: 'md:min-h-0 md:flex-col md:items-start md:gap-0 md:pt-1'}"
	>
		<span class="flex items-baseline gap-2 {empty ? '' : 'md:flex-col md:gap-0'}">
			<span class="font-display text-card font-medium">{names.long}</span>
			<span class="text-body-sm whitespace-nowrap text-text-muted">{names.date}</span>
		</span>
		{#if today}
			<span
				class="rounded-pill bg-primary px-2.5 py-0.5 text-caption font-bold whitespace-nowrap text-primary-foreground {empty
					? ''
					: 'md:mt-1'}">{m.week_today()}</span
			>
		{/if}
		{#if !empty}
			<button
				type="button"
				aria-label={m.plan_day_add({ day: names.full })}
				data-day-add={day}
				aria-haspopup="dialog"
				onclick={onadd}
				class="ml-auto inline-flex size-11 shrink-0 items-center justify-center rounded-full text-text-muted transition hover:bg-background hover:text-text focus-visible:outline-2 focus-visible:outline-primary md:mt-0.5 md:-ml-2.5 md:size-8"
			>
				<Plus class="size-4.5" aria-hidden="true" />
			</button>
		{/if}
	</header>
	<div class={empty ? 'min-w-11 flex-1' : ''}>{@render children()}</div>
</section>
