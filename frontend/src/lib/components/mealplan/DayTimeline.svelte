<script lang="ts">
	import { getDayOfWeek, parseDate } from '@internationalized/date';
	import { untrack } from 'svelte';
	import { on } from 'svelte/events';
	import type { PlanEntry } from '#lib/api/mealplan.js';
	import { dayNames, dayNumber, dayOf } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';

	/**
	 * A ruler of six weeks over the desktop's strip: a tick per day, each
	 * week's Monday named by its date, today marked, a dot where a meal is
	 * planned, and the days in view as a lens. The ruler is dates, not data:
	 * it pans only when the lens comes within a week of its ends, by just
	 * the days the strip moved, so a load changes nothing but the dots (days
	 * not loaded yet are bare ticks; days loaded before keep their dots). Dragging the lens scrolls the strip
	 * with it; a click on a day or a week's date brings it to the strip's
	 * left edge. The lens is a slider for the keyboard: Right and Up move a
	 * day on, Left and Down back, Page Up and Down a week, Home and End to
	 * the ends of the loaded days.
	 */
	let {
		days,
		today,
		entries,
		at,
		count,
		label,
		onseek,
		onrelease,
		onpick,
		onstep
	}: {
		/** The loaded days. */
		days: string[];
		today: string;
		entries: PlanEntry[];
		/** The strip's first day in view (`dayNumber`, fractional while it moves). */
		at: number;
		count: number;
		/** The days in view, as the slider's value ("22.–28. November"). */
		label: string;
		/** The strip to day `day` (`dayNumber`), following a pointer. */
		onseek: (day: number) => void;
		onrelease: () => void;
		/** `day` to the strip's left edge. */
		onpick: (day: string) => void;
		/** On by `by` days. */
		onstep: (by: number) => void;
	} = $props();

	const SPAN = 42;
	const MARGIN = 7;
	const locale = getLocale();
	// Meals per day: the loaded days' as they are now, and those of days
	// loaded before and since left behind, as last seen, so a load that
	// moves the loaded weeks on does not wipe the dots already shown.
	let seen: Record<string, number> = {};
	const planned = $derived.by(() => {
		const now: Record<string, number> = {};
		for (const day of days) now[day] = 0;
		for (const e of entries) if (e.day && e.day in now) now[e.day]++;
		seen = { ...seen, ...now };
		return seen;
	});
	const isMonday = (day: string) => getDayOfWeek(parseDate(day), 'en', 'mon') === 0;

	// The ruler's first day: two weeks before the days in view, then moved
	// only by the lens nearing an end.
	// svelte-ignore state_referenced_locally
	let from = $state(Math.round(at) - 14);
	$effect(() => {
		const lens = at;
		untrack(() => {
			if (lens < from + MARGIN) from = lens - MARGIN;
			else if (lens + count > from + SPAN - MARGIN) from = lens + count + MARGIN - SPAN;
		});
	});
	const ticks = $derived(Array.from({ length: SPAN + 1 }, (_, i) => dayOf(Math.floor(from) + i)));
	const share = (n: number) => `${(n / SPAN) * 100}%`;
	const first = $derived(days.length ? dayNumber(days[0]) : 0);
	const max = $derived(Math.max(0, days.length - count));

	let track = $state<HTMLElement>();
	let dragging = $state(false);

	function press(event: PointerEvent) {
		if (!track || event.button !== 0) return;
		const box = track.getBoundingClientRect();
		const per = box.width / SPAN;
		const week = (event.target as Element).closest<HTMLElement>('[data-week]')?.dataset.week;
		const day = from + (event.clientX - box.left) / per;
		if (week || day < at || day > at + count) {
			// A week's date or a day off the lens: there, at the left edge.
			onpick(week ?? dayOf(day));
			return;
		}
		event.preventDefault();
		const x = event.clientX;
		const start = at;
		dragging = true;
		const stop = [
			on(window, 'pointermove', (move) => onseek(start + (move.clientX - x) / per)),
			on(window, 'pointerup', up),
			on(window, 'pointercancel', up)
		];
		function up() {
			for (const off of stop) off();
			dragging = false;
			onrelease();
		}
	}

	// The slider's keys: up and right move on, Page Up and Down a week.
	const KEYS: Record<string, number> = {
		ArrowLeft: -1,
		ArrowDown: -1,
		ArrowRight: 1,
		ArrowUp: 1,
		PageDown: -7,
		PageUp: 7
	};
	function key(event: KeyboardEvent) {
		const by = KEYS[event.key];
		const end = event.key === 'Home' ? days[0] : event.key === 'End' ? days[max] : undefined;
		if (by === undefined && end === undefined) return;
		event.preventDefault();
		if (by !== undefined) onstep(by);
		else onpick(end!);
	}
</script>

<!-- The line takes the pointer, the ticks only draw; the lens is the keyboard's way. -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	bind:this={track}
	onpointerdown={press}
	data-timeline
	class="group relative h-11 cursor-pointer touch-none overflow-x-clip select-none"
>
	<div
		role="slider"
		tabindex="0"
		aria-label={m.plan_days_timeline()}
		aria-valuemin={0}
		aria-valuemax={max}
		aria-valuenow={Math.round(at - first)}
		aria-valuetext={label}
		onkeydown={key}
		style:left={share(at - from)}
		style:width={share(count)}
		class="absolute inset-y-0.5 rounded-lg bg-surface shadow-card ring-1 ring-border transition-shadow focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary dark:bg-surface-elevated dark:ring-handle {dragging
			? 'cursor-grabbing shadow-lift'
			: 'cursor-grab group-hover:ring-handle'}"
	></div>
	<!-- A day wider than the line, shifted by the ruler's part of a day. -->
	<ol
		aria-hidden="true"
		class="pointer-events-none absolute inset-y-0 left-0 grid"
		style:width={share(SPAN + 1)}
		style:transform="translateX({-((from - Math.floor(from)) / (SPAN + 1)) * 100}%)"
		style:grid-template-columns="repeat({SPAN + 1}, minmax(0, 1fr))"
	>
		{#each ticks as day (day)}
			{@const n = planned[day] ?? 0}
			<li class="relative flex flex-col items-center pt-2">
				{#if isMonday(day)}
					<span class="absolute top-1.5 bottom-1 left-0 w-px bg-border"></span>
					<span
						data-week={day}
						class="pointer-events-auto absolute bottom-0.5 left-1.5 text-micro leading-none whitespace-nowrap text-text-muted hover:text-text"
						>{dayNames(day, locale).date}</span
					>
				{/if}
				<span class="flex h-3 items-center">
					{#if day === today}
						<span data-today class="size-2.5 rounded-full bg-primary ring-3 ring-accent"></span>
					{:else if n > 0}
						<span
							data-planned
							class="rounded-full motion-safe:animate-fade-in {day < today
								? 'bg-handle'
								: 'bg-text-muted'} {n > 2 ? 'size-2' : 'size-1.5'}"
						></span>
					{:else}
						<span class="h-2 w-px rounded-full bg-handle"></span>
					{/if}
				</span>
			</li>
		{/each}
	</ol>
</div>
