<script lang="ts">
	import type { IngredientGroup, Step, StepPhoto } from '#lib/api/recipes.js';
	import StepText from './StepText.svelte';

	let {
		steps,
		groups,
		servings,
		baseServings,
		onphoto
	}: {
		steps: Step[];
		groups: IngredientGroup[];
		servings: number;
		baseServings: number;
		/** A photo phrase was activated: its photo, the phrase, and the 1-based number of its step. */
		onphoto?: (photo: StepPhoto, trigger: HTMLElement, stepNumber: number) => void;
	} = $props();
</script>

<ol class="flex flex-col gap-[22px]">
	{#each steps as step, index (index)}
		<li class="flex gap-4">
			<span
				aria-hidden="true"
				class="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent font-display text-card-sm font-semibold text-accent-foreground"
			>
				{index + 1}
			</span>
			<!-- A step reads in one sweep: the measure stops near 68 characters
			     however wide the method column grows. -->
			<p class="max-w-[68ch] pt-1 text-card-sm leading-[1.6] text-text">
				<StepText
					{step}
					{groups}
					{servings}
					{baseServings}
					onphoto={onphoto && ((photo, trigger) => onphoto(photo, trigger, index + 1))}
				/>
			</p>
		</li>
	{/each}
</ol>
