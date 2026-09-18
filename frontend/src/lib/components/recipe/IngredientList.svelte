<script lang="ts">
	import { Checkbox } from 'bits-ui';
	import Check from 'lucide-svelte/icons/check';
	import type { IngredientGroup } from '$lib/api/recipes';
	import { ingredientKey } from '$lib/recipe/checked';
	import { isChecked, toggle } from '$lib/recipe/checked.svelte';
	import { formatFactor, formatServings } from '$lib/recipe/format';
	import { formatQuantityFor } from '$lib/recipe/scale';
	import { m } from '$lib/paraglide/messages';

	let {
		recipeId,
		groups,
		servings,
		baseServings
	}: {
		recipeId: string;
		groups: IngredientGroup[];
		/** The servings the user chose (1-99). */
		servings: number;
		/** The recipe's stored servings - what every quantity is written for. */
		baseServings: number;
	} = $props();

	const scaled = $derived(servings !== baseServings);
</script>

<div class="rounded-2xl bg-surface px-6 pt-[22px] pb-2.5">
	{#if scaled}
		<p class="mb-3 text-caption font-medium text-text-muted">
			{m.servings_scaled_hint({
				servings: formatServings(servings),
				factor: formatFactor(baseServings, servings)
			})}
		</p>
	{/if}
	{#each groups as group, groupIndex (groupIndex)}
		{#if group.name}
			<h3 class="mt-5 mb-2 font-display text-[16px] font-medium text-primary italic first:mt-0">
				{group.name}
			</h3>
		{/if}
		<ul>
			{#each group.ingredients as ingredient, ingredientIndex (ingredientIndex)}
				{@const key = ingredientKey(recipeId, groupIndex, ingredientIndex)}
				{@const rowChecked = isChecked(key)}
				<li
					class="grid min-h-11 grid-cols-[22px_70px_1fr] items-center gap-2 border-b border-dashed border-border py-2 last:border-b-0 md:min-h-10"
				>
					<Checkbox.Root
						checked={rowChecked}
						onCheckedChange={() => toggle(key)}
						aria-label={ingredient.name}
						class="flex size-5 items-center justify-center rounded-full border-[1.5px] border-handle transition data-[state=checked]:border-primary data-[state=checked]:bg-primary"
					>
						{#snippet children({ checked: isRowChecked })}
							{#if isRowChecked}
								<Check class="size-3 text-primary-foreground" aria-hidden="true" />
							{/if}
						{/snippet}
					</Checkbox.Root>
					<span
						class="text-body-sm font-semibold tabular-nums {rowChecked
							? 'text-text-muted line-through'
							: 'text-text'}"
					>
						{formatQuantityFor(ingredient.quantity, baseServings, servings)}
						{ingredient.unit ?? ''}
					</span>
					<span>
						<span class="text-body {rowChecked ? 'text-text-muted line-through' : 'text-text'}">
							{ingredient.name}
						</span>
						{#if ingredient.note}
							<span class="ml-1 text-caption text-text-muted">({ingredient.note})</span>
						{/if}
					</span>
				</li>
			{/each}
		</ul>
	{/each}
</div>
