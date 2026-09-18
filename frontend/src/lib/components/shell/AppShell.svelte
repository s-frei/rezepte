<script lang="ts">
	import type { Snippet } from 'svelte';
	import { page } from '$app/state';
	import { showsBottomNav } from '$lib/shell.svelte';
	import BottomNav from './BottomNav.svelte';
	import TopBar from './TopBar.svelte';

	let { children }: { children: Snippet } = $props();

	// Pages with their own bottom action bar drop the nav on phones rather
	// than stack two floating layers on top of each other.
	const withNav = $derived(showsBottomNav(page.route.id));
</script>

<TopBar />
<main class="mx-auto max-w-[1280px] px-5 md:px-8 md:pb-8 {withNav ? 'pb-24' : 'pb-10'}">
	{@render children()}
</main>
{#if withNav}
	<BottomNav />
{/if}
