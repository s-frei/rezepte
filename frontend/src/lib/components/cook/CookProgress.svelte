<script lang="ts">
	import { m } from '$lib/paraglide/messages';

	let {
		count,
		current
	}: {
		/** Number of steps (segments). */
		count: number;
		/** Zero-based index of the step on screen. */
		current: number;
	} = $props();
</script>

<div
	role="progressbar"
	aria-label={m.cook_progress()}
	aria-valuemin={1}
	aria-valuemax={count}
	aria-valuenow={current + 1}
	aria-valuetext={m.cook_step_counter({ number: current + 1, count })}
	class="flex gap-1"
>
	<!-- Svelte 5.4+ each without an item binding: the segments are positional,
	     only the index matters. -->
	{#each { length: count }, i (i)}
		<span
			class="h-1 flex-1 rounded-pill transition-colors {i < current
				? 'bg-primary'
				: i === current
					? 'bg-primary/50'
					: 'bg-border'}"
		></span>
	{/each}
</div>
