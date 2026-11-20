<script lang="ts">
	import type { Snippet } from 'svelte';
	import Plus from '@lucide/svelte/icons/plus';
	import { dayNames } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';

	/**
	 * A day as one of the desktop's seven columns: a short header over its
	 * cards, "MO 16." on a dotted rule; today's header is an accent band and
	 * the column is outlined around its cards.
	 */
	let {
		day,
		today,
		onadd,
		children
	}: {
		day: string;
		today: boolean;
		onadd: () => void;
		children: Snippet;
	} = $props();

	const names = $derived(dayNames(day, getLocale()));
</script>

<section
	aria-label={names.full}
	aria-current={today ? 'date' : undefined}
	class="flex min-w-0 flex-col gap-2 rounded-2xl pb-2 {today ? 'outline-2 outline-primary' : ''}"
>
	<header
		class="flex items-baseline gap-1.5 border-b-2 border-dotted py-2 pr-1 pl-2.5 {today
			? 'rounded-t-2xl border-transparent bg-accent text-accent-foreground'
			: 'border-handle'}"
	>
		<span
			class="text-label font-bold tracking-[0.08em] whitespace-nowrap uppercase {today
				? ''
				: 'text-text-muted'}">{names.weekday}{today ? ` · ${m.week_today()}` : ''}</span
		>
		<span class="font-display text-heading leading-none font-medium">{names.number}</span>
		<button
			type="button"
			aria-label={m.plan_day_add({ day: names.full })}
			data-day-add={day}
			aria-haspopup="dialog"
			onclick={onadd}
			class="ml-auto inline-flex size-7 shrink-0 items-center justify-center self-center rounded-full text-text-muted transition hover:bg-surface hover:text-text focus-visible:outline-2 focus-visible:outline-primary"
		>
			<Plus class="size-4" aria-hidden="true" />
		</button>
	</header>
	<div class="flex flex-col {today ? 'px-2' : ''}">{@render children()}</div>
</section>
