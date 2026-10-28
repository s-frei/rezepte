<script lang="ts">
	import X from '@lucide/svelte/icons/x';
	import TimerRing from './TimerRing.svelte';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import { formatClock, fraction, remainingMs, type Timer } from '$lib/recipe/timers';
	import { timers } from '$lib/recipe/timers.svelte';

	let {
		timer,
		now,
		showRecipe,
		onopen
	}: {
		timer: Timer;
		now: number;
		/** A timer of another recipe carries that recipe's title. */
		showRecipe: boolean;
		onopen: () => void;
	} = $props();

	const name = $derived(m.timer_name({ number: timer.step + 1, phrase: timer.phrase }));
	const paused = $derived(timer.state === 'paused');
	const endsAt = $derived(
		new Intl.DateTimeFormat(getLocale(), { hour: '2-digit', minute: '2-digit' }).format(
			timer.endsAt ?? now
		)
	);
	const roundButton =
		'grid size-11 shrink-0 place-items-center rounded-full transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.96]';
</script>

<div class="flex min-w-0 items-center gap-2 rounded-2xl bg-background py-2 pr-2 pl-1.5">
	<button
		type="button"
		onclick={() => timers.toggle(timer.id)}
		aria-label={paused ? m.timer_resume({ name }) : m.timer_pause({ name })}
		class="{roundButton} text-time-foreground"
	>
		<TimerRing fraction={fraction(timer, now)} glyph={paused ? 'play' : 'pause'} class="size-10" />
	</button>
	<button
		type="button"
		onclick={onopen}
		class="min-w-0 flex-1 self-stretch rounded-md text-left leading-tight focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
	>
		<span class="flex items-baseline gap-2 whitespace-nowrap">
			<span data-clock class="text-card font-semibold text-time-foreground tabular-nums"
				>{formatClock(remainingMs(timer, now))}</span
			>
			<span class="text-caption font-medium text-text-muted"
				>{paused ? m.timer_paused() : m.timer_ends_at({ time: endsAt })}</span
			>
		</span>
		<span class="flex text-caption font-medium whitespace-nowrap text-text-muted">
			<span data-timer-name class="max-w-full shrink-0 truncate">{name}</span>
			{#if showRecipe}<span class="min-w-0 truncate">&nbsp;· {timer.recipeTitle}</span>{/if}
		</span>
	</button>
	<button
		type="button"
		onclick={() => timers.stop(timer.id)}
		aria-label={m.timer_stop({ name })}
		class="{roundButton} bg-surface text-text"
	>
		<X class="size-5" aria-hidden="true" />
	</button>
</div>
