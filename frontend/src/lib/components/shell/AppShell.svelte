<script lang="ts">
	import type { Snippet } from 'svelte';
	import { page } from '$app/state';
	import { navPlace } from '$lib/shell.svelte';
	import TimerIndicator from '$lib/components/timer/TimerIndicator.svelte';
	import { timers } from '$lib/recipe/timers.svelte';
	import BottomNav from './BottomNav.svelte';
	import TopBar from './TopBar.svelte';

	let { children }: { children: Snippet } = $props();

	// The phone's nav shows only where one of its places is current; pages
	// with their own action bar, and the error page, go without it.
	const place = $derived(navPlace(page.route.id, page.error !== null));
	const withNav = $derived(place !== null);
	// While a timer shows, phones make room for the island (see
	// `--timer-island` in app.css).
	$effect(() => {
		document.documentElement.toggleAttribute('data-timer-island', timers.list.length > 0);
		return () => document.documentElement.removeAttribute('data-timer-island');
	});
</script>

<TopBar />
<main
	class="mx-auto max-w-[1280px] px-5 md:px-8 md:pb-8 {withNav
		? 'pb-20'
		: 'pb-10'} pt-[var(--timer-island,0px)]"
>
	{@render children()}
</main>
<TimerIndicator />
{#if place}
	<BottomNav {place} />
{/if}
