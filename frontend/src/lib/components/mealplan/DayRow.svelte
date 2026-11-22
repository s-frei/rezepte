<script lang="ts">
	import type { Snippet } from 'svelte';
	import Plus from '@lucide/svelte/icons/plus';
	import { dayNames } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';

	/**
	 * A phone's day as a card: the name and date over its entries, the "+"
	 * beside them. An empty day is one line, its name and the dashed "+ Plan
	 * a meal". Today's card is outlined and wears a "Today" pill.
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

<!-- Focusable for a stamp's jump to the day (`tabindex="-1"`, no stop of its
     own). Positioned: an empty day's zone covers it during a drag (`EntryZone`). -->
<section
	data-day={day}
	tabindex="-1"
	aria-label={names.full}
	aria-current={today ? 'date' : undefined}
	id={today ? 'today' : undefined}
	class="relative rounded-2xl bg-surface shadow-card {today
		? 'outline-2 outline-primary'
		: 'outline-none focus-visible:outline-2 focus-visible:outline-primary/60 focus-visible:outline-solid'} {empty
		? 'flex items-center gap-3 py-1.5 pr-3 pl-4'
		: 'pt-1.5 pr-3 pb-3 pl-4'}"
>
	<header class="flex min-h-11 items-center gap-2 {empty ? 'flex-none' : ''}">
		<span class="flex items-baseline gap-2">
			<span class="font-display text-card font-medium">{names.long}</span>
			<span class="text-body-sm whitespace-nowrap text-text-muted">{names.date}</span>
		</span>
		{#if today}
			<span
				class="rounded-pill bg-primary px-2.5 py-0.5 text-caption font-bold whitespace-nowrap text-primary-foreground"
				>{m.week_today()}</span
			>
		{/if}
		{#if !empty}
			<button
				type="button"
				aria-label={m.plan_day_add({ day: names.full })}
				data-day-add={day}
				aria-haspopup="dialog"
				onclick={onadd}
				class="ml-auto inline-flex size-11 shrink-0 items-center justify-center rounded-full text-text-muted transition hover:bg-background hover:text-text focus-visible:outline-2 focus-visible:outline-primary"
			>
				<Plus class="size-4.5" aria-hidden="true" />
			</button>
		{/if}
	</header>
	<div class={empty ? 'min-w-11 flex-1' : ''}>{@render children()}</div>
</section>
