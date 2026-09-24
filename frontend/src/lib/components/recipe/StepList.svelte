<script lang="ts">
	import type { IngredientGroup, Step } from '$lib/api/recipes';
	import StepText from './StepText.svelte';

	let {
		steps,
		groups,
		servings,
		baseServings
	}: {
		steps: Step[];
		groups: IngredientGroup[];
		servings: number;
		baseServings: number;
	} = $props();
</script>

<ol class="flex flex-col gap-[22px]">
	{#each steps as step, index (index)}
		<li class="flex gap-4">
			<span
				aria-hidden="true"
				class="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent font-display text-[16px] font-semibold text-accent-foreground"
			>
				{index + 1}
			</span>
			<p class="pt-1 text-[16px] leading-[1.6] text-text">
				<StepText {step} {groups} {servings} {baseServings} />
			</p>
		</li>
	{/each}
</ol>
