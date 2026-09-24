<script lang="ts">
	import type { IngredientGroup, Step } from '$lib/api/recipes';
	import { segmentStep } from '$lib/recipe/references';

	let {
		step,
		groups,
		servings,
		baseServings
	}: {
		step: Step;
		groups: IngredientGroup[];
		/** The servings the user chose. */
		servings: number;
		/** The recipe's stored servings - what every quantity is written for. */
		baseServings: number;
	} = $props();

	const segments = $derived(segmentStep(step, groups, servings, baseServings));

	// No ARIA: the visible text is already the accessible text and reads
	// correctly as "Saft (400 ml) mit Zucker (100 g) aufkochen". A label would
	// only get in the way. Sizes stay in em so the annotation grows with cook
	// mode's display type. A quantity never breaks across lines: "(100" at the
	// end of one line and "g)" at the start of the next is the one thing a cook
	// glancing at the step must not have to piece together, and at cook mode's
	// size it happened on every longer step. (Comment kept here, not as an HTML
	// comment above the markup, so it never ships into the rendered DOM.)
</script>

{#each segments as segment, index (index)}{segment.text}{#if segment.quantity}<span
			class="ml-[0.2em] font-semibold whitespace-nowrap text-primary">({segment.quantity})</span
		>{/if}{/each}
