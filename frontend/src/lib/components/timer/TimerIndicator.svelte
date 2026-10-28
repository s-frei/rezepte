<script lang="ts">
	import Bell from '@lucide/svelte/icons/bell';
	import { resolve } from '$app/paths';
	import TimerRing from './TimerRing.svelte';
	import { m } from '$lib/paraglide/messages';
	import { byEnding, formatClock, fraction, remainingMs } from '$lib/recipe/timers';
	import { timers } from '$lib/recipe/timers.svelte';

	const first = $derived(byEnding(timers.list, timers.now)[0]);
	const more = $derived(timers.list.length - 1);
	const ringing = $derived(first?.state === 'ringing');
	const name = $derived(
		first ? m.timer_name({ number: first.step + 1, phrase: first.phrase }) : ''
	);
	const clock = $derived(first ? formatClock(remainingMs(first, timers.now)) : '');
</script>

<!-- Below `md` an island floating at the top center (AppShell pads the page
     by its height); from `md` the cook-mode capsule at the bottom right,
     which the Toaster's offset keeps clear of. A ringing first timer turns
     it orange with "Aus" beside the link. -->
{#if first}
	<div
		data-timer-indicator
		class="fixed top-[calc(10px+env(safe-area-inset-top))] left-1/2 z-40 flex max-w-[calc(100vw-16px)] -translate-x-1/2 items-center rounded-pill shadow-lift md:top-auto md:right-6 md:bottom-6 md:left-auto md:max-w-[480px] md:translate-x-0 {ringing
			? 'bg-primary text-primary-foreground'
			: 'bg-inverse text-inverse-foreground md:bg-user-teal md:text-time-foreground'}"
	>
		<a
			href={resolve(`/recipes/${first.slug}/cook?step=${first.step + 1}`)}
			aria-label={`${m.timer_open({ name: `${first.recipeTitle} · ${name}` })} · ${ringing ? m.timer_done() : clock}`}
			class="flex h-11 min-w-0 items-center gap-2 rounded-pill {ringing
				? 'pr-2'
				: 'pr-3.5'} pl-1.5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			{#if ringing}
				<Bell class="ml-1 size-5 shrink-0" aria-hidden="true" />
				<!-- The recipe is in the link's label; on screen the name must fit. -->
				<span data-timer-name class="truncate text-caption font-semibold md:text-body-sm"
					>{m.timer_done()} · {name}</span
				>
			{:else}
				<TimerRing
					fraction={fraction(first, timers.now)}
					class="size-8 text-user-teal md:text-time-foreground"
				/>
				<span class="text-card-sm font-semibold tabular-nums">{clock}</span>
				<span class="truncate text-caption font-medium opacity-85">{first.recipeTitle}</span>
				{#if more > 0}
					<span
						class="shrink-0 rounded-pill bg-inverse-foreground/20 px-1.5 py-0.5 text-micro font-semibold md:bg-time-foreground md:text-user-teal"
						>{m.timer_more({ count: more })}</span
					>
				{/if}
			{/if}
		</a>
		{#if ringing}
			<span role="alert" class="sr-only">{m.timer_done()} · {first.recipeTitle} · {name}</span>
			<button
				type="button"
				onclick={() => timers.stop(first.id)}
				class="relative mr-1 h-9 shrink-0 rounded-pill bg-primary-foreground px-4 text-body-sm font-semibold text-primary transition after:absolute after:-inset-1 after:content-[''] hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-foreground active:scale-[.98]"
			>
				{m.timer_off()}
			</button>
		{/if}
	</div>
{/if}
