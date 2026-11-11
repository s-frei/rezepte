<script lang="ts">
	import type { Snippet } from 'svelte';

	// One settings card: a titled surface the heading names for assistive tech.
	let { title, intro, children }: { title: string; intro?: string; children: Snippet } = $props();

	const id = $props.id();
</script>

<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby={id}>
	<h2 {id} class={['font-display text-heading font-medium', !intro && 'mb-4']}>{title}</h2>
	{#if intro}
		<p class="mt-1 mb-4 max-w-[60ch] text-caption text-text-muted">{intro}</p>
	{/if}
	{@render children()}
</section>
