<script lang="ts">
	import { tick } from 'svelte';
	import Bell from '@lucide/svelte/icons/bell';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import { afterNavigate } from '$app/navigation';
	import TimerBanner from './TimerBanner.svelte';
	import TimerRow from './TimerRow.svelte';
	import TimerSummary from './TimerSummary.svelte';
	import { isField } from '$lib/components/shell/typing';
	import { bottomNav } from '$lib/shell.svelte';
	import { m } from '$lib/paraglide/messages';
	import { byEnding } from '$lib/recipe/timers';
	import { timers } from '$lib/recipe/timers.svelte';

	/** A phone's bottom nav is on this page: the player sits above it. */
	let { aboveNav }: { aboveNav: boolean } = $props();

	const list = $derived(byEnding(timers.list, timers.now));
	const first = $derived(list[0]);
	const ringing = $derived(first?.state === 'ringing');
	const name = $derived(
		first ? m.timer_name({ number: first.step + 1, phrase: first.phrase }) : ''
	);

	// Every timer, ringing ones first as banners, above the capsule. Closed
	// by the capsule again, Escape, a tap elsewhere and every navigation (a
	// row's link included).
	let open = $state(false);
	// On a phone the player is one stack with the nav: as wide as the bar and
	// just above it, floating (the bar at 95%, lifted) or docked, and moving
	// with its morph; without a nav, as wide as one would be, on the edge.
	const place = $derived(
		!aboveNav
			? 'inset-x-2 bottom-[calc(0.5rem+env(safe-area-inset-bottom))]'
			: bottomNav.docked
				? 'inset-x-2 bottom-[calc(4.5rem+env(safe-area-inset-bottom))]'
				: 'inset-x-[calc(0.5rem+(100vw-1rem)*0.025)] bottom-[calc(0.75rem+0.95*(4rem+env(safe-area-inset-bottom))+env(safe-area-inset-bottom)+0.5rem)]'
	);
	// The software keyboard is up: on a phone the player steps aside like the nav.
	let typing = $state(false);
	let root = $state<HTMLElement>();
	let toggle = $state<HTMLButtonElement>();

	afterNavigate(() => {
		open = false;
	});

	function onpointerdown(event: PointerEvent) {
		if (open && event.target instanceof Node && !root?.contains(event.target)) {
			open = false;
		}
	}

	async function onkeydown(event: KeyboardEvent) {
		if (open && event.key === 'Escape') {
			event.preventDefault();
			open = false;
			await tick();
			toggle?.focus();
		}
	}
</script>

<svelte:window
	{onpointerdown}
	{onkeydown}
	onfocusin={(event) => (typing = isField(event.target))}
	onfocusout={() => (typing = false)}
/>

<!-- Below `md` a player across the bottom, above the nav where there is one
     (AppShell ends the page above it); from `md` the cook-mode capsule at
     the bottom right. The Toaster's offsets keep toasts clear of both. A
     ringing first timer turns it orange with "Aus" beside it. Only the nav
     shares the bottom with it, so no page chrome has to know it exists.
     Named for view transitions like the nav, so the overview's cards glide
     under it, not over it. -->
{#if first}
	<div
		data-timer-indicator
		bind:this={root}
		class="fixed z-40 flex flex-col-reverse gap-2 transition-[left,right,bottom] duration-500 ease-[cubic-bezier(0.3,1.3,0.5,1)] [view-transition-name:timers] motion-reduce:transition-none {place} {bottomNav.still
			? 'transition-none'
			: ''} md:inset-x-auto md:right-6 md:bottom-6 md:items-end {typing ? 'max-md:hidden' : ''}"
	>
		<div
			class="flex items-center rounded-pill shadow-lift md:max-w-[480px] {ringing
				? 'ringing-halo bg-primary text-primary-foreground'
				: 'bg-user-teal text-time-foreground'}"
		>
			<!-- The chevron points where the list unfolds, up, and back once it is open. -->
			<button
				type="button"
				bind:this={toggle}
				onclick={() => (open = !open)}
				aria-expanded={open}
				aria-controls="timer-panel"
				class="flex h-11 min-w-0 flex-1 items-center gap-2 rounded-pill md:flex-none {ringing
					? 'pr-2'
					: 'pr-3'} hover-tint pl-1.5 text-left transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				{#if ringing}
					<Bell class="ml-1 size-5 shrink-0 bell-swing" aria-hidden="true" />
					<!-- Two lines, so the name stays whole at 320 px: the recipe
					     title beside "Fertig" is what gives way. -->
					<span class="flex min-w-0 flex-col leading-tight">
						<span class="truncate text-caption font-semibold"
							>{m.timer_done()} · {first.recipeTitle}</span
						>
						<span data-timer-name class="truncate text-micro font-medium opacity-90">{name}</span>
					</span>
					{#if list.length > 1}
						<span class="sr-only">{m.timer_more({ count: list.length - 1 })}</span>
					{/if}
					<ChevronDown
						class="ml-auto size-[18px] shrink-0 motion-safe:transition-transform {open
							? ''
							: 'rotate-180'}"
						aria-hidden="true"
					/>
				{:else}
					<TimerSummary
						timer={first}
						now={timers.now}
						title={first.recipeTitle}
						detail={name}
						more={list.length - 1}
						ringClass="text-time-foreground"
						chevronClass={open ? '' : 'rotate-180'}
					/>
				{/if}
			</button>
			{#if ringing}
				<span role="alert" class="sr-only">{m.timer_done()} · {first.recipeTitle} · {name}</span>
				<button
					type="button"
					onclick={() => timers.stop(first.id)}
					class="relative mr-1 h-9 shrink-0 rounded-pill bg-primary-foreground hover-tint px-4 text-body-sm font-semibold text-primary transition after:absolute after:-inset-1 after:content-[''] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-foreground active:scale-[.98]"
				>
					{m.timer_off()}
				</button>
			{/if}
		</div>
		<div
			id="timer-panel"
			class="{open
				? 'flex'
				: 'hidden'} max-h-[min(70dvh,560px)] flex-col gap-2 overflow-y-auto overscroll-contain rounded-3xl bg-user-teal p-2 shadow-lift md:w-[400px]"
		>
			{#each list as t (t.id)}
				{#if t.state === 'ringing'}
					<TimerBanner timer={t} showRecipe />
				{:else}
					<TimerRow timer={t} now={timers.now} showRecipe link />
				{/if}
			{/each}
		</div>
	</div>
{/if}
