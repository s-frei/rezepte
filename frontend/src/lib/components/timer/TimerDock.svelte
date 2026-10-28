<script lang="ts">
	import { onMount, tick } from 'svelte';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import ChevronUp from '@lucide/svelte/icons/chevron-up';
	import TimerRing from './TimerRing.svelte';
	import TimerRow from './TimerRow.svelte';
	import { m } from '$lib/paraglide/messages';
	import { byEnding, formatClock, fraction, remainingMs, type Timer } from '$lib/recipe/timers';
	import { timers } from '$lib/recipe/timers.svelte';
	import { pushUnavailable } from '$lib/push.svelte';

	let { recipeId, onopen }: { recipeId: string; onopen: (t: Timer) => void } = $props();

	// A ringing timer is the banner's, not the dock's.
	const list = $derived(byEnding(timers.list, timers.now).filter((t) => t.state !== 'ringing'));
	const first = $derived(list[0]);
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
	<div data-timer-dock class="flex shrink-0 flex-col items-center gap-1.5 px-5 pb-2.5 md:px-8">
		{#if !expanded}
			<button
				type="button"
				bind:this={capsule}
				onclick={() => setExpanded(true)}
				aria-label={m.timer_expand()}
				aria-expanded="false"
				class="inline-flex h-12 max-w-full items-center gap-2.5 rounded-pill bg-user-teal pr-3.5 pl-2 text-time-foreground transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] md:hidden"
			>
				<TimerRing fraction={fraction(first, timers.now)} class="size-8" />
				<span data-clock class="text-body-lg font-semibold tabular-nums"
					>{formatClock(remainingMs(first, timers.now))}</span
				>
				<span class="truncate text-caption font-medium"
					>{m.editor_step_number({ number: first.step + 1 })}</span
				>
				{#if list.length > 1}
					<span
						class="rounded-pill bg-time-foreground px-1.5 py-0.5 text-micro font-semibold text-user-teal"
						>{m.timer_more({ count: list.length - 1 })}</span
					>
				{/if}
				<ChevronUp class="size-[18px] shrink-0" aria-hidden="true" />
			</button>
		{/if}
		<!-- Below `md` the panel on `accent`, only while expanded; from `md`
		     the bare rows, two side by side, always. -->
		<div
			class="{expanded
				? 'flex'
				: 'hidden'} max-h-[40dvh] w-full flex-col gap-2 overflow-y-auto rounded-3xl bg-accent px-2 pt-1.5 pb-2 md:flex md:max-h-none md:max-w-[760px] md:flex-row md:flex-wrap md:justify-center md:overflow-visible md:bg-transparent md:p-0"
		>
			<div class="flex items-center justify-between pl-2.5 md:hidden">
				<p class="text-caption font-semibold text-text-muted">
					{list.length === 1 ? m.timer_count_one() : m.timer_count({ count: list.length })}
				</p>
				<button
					type="button"
					bind:this={collapse}
					onclick={() => setExpanded(false)}
					aria-label={m.timer_collapse()}
					aria-expanded="true"
					class="grid size-11 place-items-center rounded-full text-text-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<ChevronDown class="size-[18px]" aria-hidden="true" />
				</button>
			</div>
			{#each list as t (t.id)}
				<div class="md:w-[calc(50%-4px)]">
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
