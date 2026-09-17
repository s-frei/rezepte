<script lang="ts">
	import type { Snippet } from 'svelte';
	import BottomNav from './BottomNav.svelte';
	import TopBar from './TopBar.svelte';

	let { children }: { children: Snippet } = $props();
</script>

<TopBar />
<main class="mx-auto max-w-[1280px] px-5 pb-24 md:px-8 md:pb-8">
	{@render children()}
</main>
<BottomNav />
