<script lang="ts">
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import { dayNames } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';

	/**
	 * A phone's fold over this week's days gone by: "Mo–Do · 6 Einträge",
	 * opened in place. The day cards it hides follow it in the page.
	 */
	let {
		days,
		count,
		expanded = $bindable(false),
		controls
	}: {
		days: string[];
		count: number;
		expanded?: boolean;
		/** The id of what it shows and hides. */
		controls: string;
	} = $props();

	const locale = getLocale();
	const span = $derived(
		days.length === 1
			? dayNames(days[0], locale).weekday
			: `${dayNames(days[0], locale).weekday}–${dayNames(days.at(-1)!, locale).weekday}`
	);
	const what = $derived(
		expanded
			? m.plan_past_days_hide()
			: count === 0
				? m.plan_past_days_none()
				: count === 1
					? m.plan_past_days_one()
					: m.plan_past_days({ count })
	);
</script>

<button
	type="button"
	aria-expanded={expanded}
	aria-controls={controls}
	onclick={() => (expanded = !expanded)}
	class="flex min-h-12 w-full items-center gap-1.5 rounded-2xl bg-surface px-4 text-left text-body-sm whitespace-nowrap shadow-card focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
>
	<span class="font-display text-card-sm font-medium">{span}</span>
	<span class="text-text-muted">· {what}</span>
	<ChevronDown
		class="ml-auto size-4 text-text-muted transition-transform motion-reduce:transition-none {expanded
			? 'rotate-180'
			: ''}"
		aria-hidden="true"
	/>
</button>
