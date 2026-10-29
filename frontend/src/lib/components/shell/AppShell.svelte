<script lang="ts">
	import type { Snippet } from 'svelte';
	import { afterNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { bottomNav, navPlace } from '$lib/shell.svelte';
	import TimerIndicator from '$lib/components/timer/TimerIndicator.svelte';
	import { timers } from '$lib/recipe/timers.svelte';
	import BottomNav from './BottomNav.svelte';
	import TopBar from './TopBar.svelte';
	import { isField } from './typing';

	let { children }: { children: Snippet } = $props();

	// The phone's nav shows only where one of its places is current; pages
	// with their own action bar, and the error page, go without it.
	const place = $derived(navPlace(page.route.id, page.error !== null));
	const withNav = $derived(place !== null);
	// A running timer's player sits above the phone's nav (or the bottom
	// edge), and from `md` in the bottom right corner: the page ends above it.
	const bottom = $derived(
		timers.list.length > 0
			? `md:pb-24 ${withNav ? 'pb-36' : 'pb-24'}`
			: `md:pb-8 ${withNav ? 'pb-20' : 'pb-10'}`
	);

	// A navigation can take the focused field away without a `focusout`, so
	// it reads the focus afresh; one that keeps the focus keeps `typing`.
	afterNavigate(() => {
		bottomNav.typing = isField(document.activeElement);
	});
</script>

<svelte:window
	onfocusin={(event) => (bottomNav.typing = isField(event.target))}
	onfocusout={() => (bottomNav.typing = false)}
/>

<TopBar />
<main class="mx-auto max-w-[1280px] px-5 md:px-8 {bottom}">
	{@render children()}
</main>
<TimerIndicator aboveNav={withNav} />
{#if place}
	<BottomNav {place} />
{/if}
