<script lang="ts">
	import type { Snippet } from 'svelte';
	import { page } from '$app/state';
	import { navPlace } from '$lib/shell.svelte';
	import BottomNav from './BottomNav.svelte';
	import TopBar from './TopBar.svelte';

	let { children }: { children: Snippet } = $props();

	// The phone's nav shows only where one of its places is current; pages
	// with their own action bar, and the error page, go without it.
	const place = $derived(navPlace(page.route.id, page.error !== null));
	const withNav = $derived(place !== null);
</script>

<TopBar />
<main class="mx-auto max-w-[1280px] px-5 md:px-8 md:pb-8 {withNav ? 'pb-20' : 'pb-10'}">
	{@render children()}
</main>
{#if place}
	<BottomNav />
{/if}
