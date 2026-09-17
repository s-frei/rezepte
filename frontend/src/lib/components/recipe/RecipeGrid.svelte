<script lang="ts">
	import type { Snippet } from 'svelte';

	// Layout only - the caller decides what goes in each cell (real
	// `RecipeCard`s or skeleton placeholders while loading), so both share
	// the exact same grid.
	let { children }: { children: Snippet } = $props();
</script>

<div class="grid grid-cols-2 gap-3.5 md:grid-cols-4 md:gap-[22px]">
	{@render children()}
</div>
