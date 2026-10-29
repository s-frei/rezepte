<script lang="ts">
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import TimerRing from './TimerRing.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { formatClock, fraction, remainingMs, type Timer } from '#lib/recipe/timers.js';

	let {
		timer,
		now,
		title,
		detail,
		more,
		chevronClass = '',
		ringClass = ''
	}: {
		timer: Timer;
		now: number;
		/** Which timer it is, above how it stands. */
		title: string;
		detail: string;
		/** The other timers, as "+N". */
		more: number;
		/** The chevron points down; `rotate-180` turns it up. */
		chevronClass?: string;
		ringClass?: string;
	} = $props();
</script>

<!-- The inside of a collapsed timer capsule: the timer with the least time
     left, its ring, its clock and two lines saying which one it is. "+N"
     sits on the ring like a badge, so at 320 px the name keeps its width. -->
<span class="relative shrink-0">
	<TimerRing fraction={fraction(timer, now)} class="size-8 {ringClass}" />
	{#if more > 0}
		<span
			class="absolute -right-2 -bottom-0.5 rounded-pill bg-time-foreground px-1 text-micro leading-4 font-semibold text-user-teal ring-2 ring-user-teal"
			>{m.timer_more({ count: more })}</span
		>
	{/if}
</span>
<span data-clock class="text-body-lg font-semibold tabular-nums"
	>{formatClock(remainingMs(timer, now))}</span
>
<span class="flex min-w-0 flex-1 flex-col text-left leading-tight">
	<span data-timer-name class="truncate text-caption font-semibold">{title}</span>
	<span class="truncate text-micro font-medium opacity-80">{detail}</span>
</span>
<ChevronDown
	class="size-[18px] shrink-0 motion-safe:transition-transform {chevronClass}"
	aria-hidden="true"
/>
