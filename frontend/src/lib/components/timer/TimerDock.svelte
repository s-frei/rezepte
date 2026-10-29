<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { backOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade, fly } from 'svelte/transition';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import TimerRow from './TimerRow.svelte';
	import TimerSummary from './TimerSummary.svelte';
	import { timerStatus } from './status';
	import { m } from '$lib/paraglide/messages';
	import { byEnding, formatClock, remainingMs, type Timer } from '$lib/recipe/timers';
	import { timers } from '$lib/recipe/timers.svelte';
	import { pushUnavailable } from '$lib/push.svelte';

	let { recipeId, onopen }: { recipeId: string; onopen: (t: Timer) => void } = $props();

	// A ringing timer is the banner's, not the dock's.
	const list = $derived(byEnding(timers.list, timers.now).filter((t) => t.state !== 'ringing'));
	const first = $derived(list[0]);
	// Rises in when the first timer starts; there from the first render when
	// cook mode opens with timers running.
	const rise = $derived(prefersReducedMotion.current ? 0 : 250);
	// Phones only: wide screens always show the rows. Held for this visit.
	let expanded = $state(false);
	let capsule = $state<HTMLButtonElement>();
	let collapse = $state<HTMLButtonElement>();

	// The pressed button goes away, so focus follows to its counterpart.
	async function setExpanded(value: boolean) {
		expanded = value;
		await tick();
		(value ? collapse : capsule)?.focus();
	}

	// What a screen reader hears: a start, a pause and a resume, once each -
	// never the countdown. The end is the banner's `role="alert"`.
	let announcement = $state('');
	onMount(() =>
		timers.listen((event, t) => {
			const name = m.timer_name({ number: t.step + 1, phrase: t.phrase });
			if (event === 'pause') {
				announcement = `${name} · ${m.timer_paused()}`;
			} else if (event === 'start' || event === 'resume') {
				announcement = `${name} · ${formatClock(remainingMs(t, Date.now()))}`;
			}
		})
	);
</script>

<p class="sr-only" aria-live="polite">{announcement}</p>

{#if first}
	<div
		data-timer-dock
		in:fly={{ y: 16, duration: rise, easing: backOut }}
		out:fade={{ duration: 150 }}
		class="flex shrink-0 flex-col items-center gap-1.5 px-5 pb-2.5 md:px-8"
	>
		{#if !expanded}
			<button
				type="button"
				bind:this={capsule}
				onclick={() => setExpanded(true)}
				aria-label={m.timer_expand()}
				aria-expanded="false"
				class="flex h-12 w-full max-w-[392px] items-center gap-2.5 rounded-pill bg-user-teal hover-tint pr-3.5 pl-2 text-time-foreground transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] md:hidden"
			>
				<TimerSummary
					timer={first}
					now={timers.now}
					title={m.timer_name({ number: first.step + 1, phrase: first.phrase })}
					detail={first.recipeId === recipeId
						? timerStatus(first, timers.now)
						: `${timerStatus(first, timers.now)} · ${first.recipeTitle}`}
					more={list.length - 1}
					chevronClass="rotate-180"
				/>
			</button>
		{/if}
		<!-- Below `md` the panel on `user-teal`, the capsule's own color, only
		     while expanded; from `md` the bare rows, two side by side, always -
		     one alone as wide as the footer under it. Expanding unfolds it
		     upward where the capsule was; the always-open rows stay still. -->
		<div
			class="{expanded
				? 'flex max-md:origin-bottom max-md:motion-safe:animate-unfold'
				: 'hidden'} max-h-[40dvh] w-full max-w-[392px] flex-col gap-2 overflow-y-auto rounded-3xl bg-user-teal px-2 pt-1.5 pb-2 md:flex md:max-h-none md:max-w-[760px] md:flex-row md:flex-wrap md:justify-center md:overflow-visible md:bg-transparent md:p-0"
		>
			<div class="flex items-center justify-between pl-2.5 md:hidden">
				<p class="text-caption font-semibold text-time-foreground">
					{list.length === 1 ? m.timer_count_one() : m.timer_count({ count: list.length })}
				</p>
				<button
					type="button"
					bind:this={collapse}
					onclick={() => setExpanded(false)}
					aria-label={m.timer_collapse()}
					aria-expanded="true"
					class="grid size-11 place-items-center rounded-full hover-tint text-time-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<ChevronDown class="size-[18px]" aria-hidden="true" />
				</button>
			</div>
			{#each list as t (t.id)}
				<div class={list.length === 1 ? 'md:w-full md:max-w-[392px]' : 'md:w-[calc(50%-4px)]'}>
					<TimerRow
						timer={t}
						now={timers.now}
						showRecipe={t.recipeId !== recipeId}
						onopen={() => onopen(t)}
					/>
				</div>
			{/each}
		</div>
		{#if pushUnavailable()}
			<p data-push-unavailable class="text-center text-caption text-text-muted">
				{m.timer_push_unavailable()}
			</p>
		{/if}
	</div>
{/if}
