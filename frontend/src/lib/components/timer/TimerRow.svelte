<script lang="ts">
	import X from '@lucide/svelte/icons/x';
	import { resolve } from '$app/paths';
	import TimerRing from './TimerRing.svelte';
	import { timerStatus } from './status';
	import { m } from '#lib/paraglide/messages.js';
	import { cookPath, formatClock, fraction, remainingMs, type Timer } from '#lib/recipe/timers.js';
	import { timers } from '#lib/recipe/timers.svelte.js';

	let {
		timer,
		now,
		showRecipe,
		onopen,
		link = false
	}: {
		timer: Timer;
		now: number;
		/** A timer of another recipe carries that recipe's title. */
		showRecipe: boolean;
		/** Opens the timer's step in place (cook mode)... */
		onopen?: () => void;
		/** ...or links to its cook route (outside cook mode). */
		link?: boolean;
	} = $props();

	const name = $derived(m.timer_name({ number: timer.step + 1, phrase: timer.phrase }));
	const paused = $derived(timer.state === 'paused');
	const roundButton =
		'grid size-11 shrink-0 place-items-center rounded-full transition hover-tint focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.96]';
	// The label's own padding holds the text off the hover tint's edge,
	// which ran flush to the clock; a negative margin of the same 4px lets
	// it reach into the row's padding, so no row grows or shrinks.
	const openClass =
		'hover-tint min-w-0 flex-1 self-stretch -my-1 rounded-xl px-3 py-1 text-left leading-tight focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
</script>

{#snippet label()}
	<span class="flex items-baseline gap-2 whitespace-nowrap">
		<span data-clock class="text-card font-semibold text-time-foreground tabular-nums"
			>{formatClock(remainingMs(timer, now))}</span
		>
		<span class="text-caption font-medium text-text-muted">{timerStatus(timer, now)}</span>
	</span>
	<span data-timer-name class="block truncate text-caption font-medium text-text-muted">{name}</span
	>
	<!-- Another recipe's title on a line of its own, so it never cuts the name. -->
	{#if showRecipe}
		<span class="block truncate text-micro font-medium text-text-muted opacity-80"
			>{timer.recipeTitle}</span
		>
	{/if}
{/snippet}

<div class="flex h-full min-w-0 items-center gap-1 rounded-2xl bg-background py-2 pr-2 pl-1.5">
	<button
		type="button"
		onclick={() => timers.toggle(timer.id)}
		aria-label={paused ? m.timer_resume({ name }) : m.timer_pause({ name })}
		class="{roundButton} text-time-foreground"
	>
		<TimerRing fraction={fraction(timer, now)} glyph={paused ? 'play' : 'pause'} class="size-10" />
	</button>
	{#if link}
		<a href={resolve(cookPath(timer))} class="flex flex-col justify-center {openClass}"
			>{@render label()}</a
		>
	{:else}
		<button type="button" onclick={onopen} class={openClass}>{@render label()}</button>
	{/if}
	<button
		type="button"
		onclick={() => timers.stop(timer.id)}
		aria-label={m.timer_stop({ name })}
		class="{roundButton} bg-surface text-text"
	>
		<X class="size-5" aria-hidden="true" />
	</button>
</div>
