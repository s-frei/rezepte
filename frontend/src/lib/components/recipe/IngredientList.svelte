<script lang="ts">
	import { Checkbox } from 'bits-ui';
	import Check from '@lucide/svelte/icons/check';
	import Copy from '@lucide/svelte/icons/copy';
	import type { IngredientGroup } from '$lib/api/recipes';
	import { ingredientKey } from '$lib/recipe/checked';
	import { isChecked, toggle } from '$lib/recipe/checked.svelte';
	import { formatFactor, formatServings } from '$lib/recipe/format';
	import { shoppingList } from '$lib/recipe/ingredient-text';
	import { formatQuantityFor } from '$lib/recipe/scale';
	import { copyText } from '$lib/recipe/share.svelte';
	import { m } from '$lib/paraglide/messages';

	let {
		recipeId,
		groups,
		servings,
		baseServings,
		copyable = false
	}: {
		recipeId: string;
		groups: IngredientGroup[];
		/** The servings the user chose (1-99). */
		servings: number;
		/** The recipe's stored servings - what every quantity is written for. */
		baseServings: number;
		/** Ends the card in a tear-off shopping list. */
		copyable?: boolean;
	} = $props();

	const uid = $props.id();
	const scaled = $derived(servings !== baseServings);
	const checkedAt = (g: number, i: number) => isChecked(ingredientKey(recipeId, g, i));
	const total = $derived(groups.reduce((sum, group) => sum + group.ingredients.length, 0));
	const open = $derived(
		groups.reduce(
			(sum, group, g) => sum + group.ingredients.filter((_, i) => !checkedAt(g, i)).length,
			0
		)
	);
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
	{#if copyable}
		<!-- One sheet with the list: the notches are page-colored circles centered on the tear line, so card and stub each lose half. -->
		<div class="relative -mx-6 mt-2.5 -mb-2.5">
			<div
				class="mx-[18px] h-1 bg-[radial-gradient(circle,var(--color-handle)_1.6px,transparent_2px)] bg-[length:9px_4px] bg-repeat-x"
				aria-hidden="true"
			></div>
			<span
				class="absolute top-0.5 -left-[11px] size-[22px] -translate-y-1/2 rounded-full bg-background"
				aria-hidden="true"
			></span>
			<span
				class="absolute top-0.5 -right-[11px] size-[22px] -translate-y-1/2 rounded-full bg-background"
				aria-hidden="true"
			></span>
			<button
				type="button"
				disabled={open === 0}
				aria-label={m.ingredients_slip_copy()}
				aria-describedby="{uid}-slip"
				onclick={() =>
					copyText(
						shoppingList(groups, baseServings, servings, checkedAt),
						m.ingredients_slip_copied()
					)}
				class="flex w-full items-center gap-4 rounded-b-2xl px-6 pt-4 pb-5 text-left focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
			>
				<span class="min-w-0 flex-1">
					<span class="block font-display text-[17px] font-medium text-primary italic">
						{m.ingredients_slip_title()}
					</span>
					<span id="{uid}-slip" class="block text-caption text-text-muted">
						{open === 0
							? m.ingredients_slip_done()
							: m.ingredients_slip_count({ open, total, servings: formatServings(servings) })}
					</span>
				</span>
				{#if open > 0}
					<span
						class="flex size-10 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground transition active:scale-[.96]"
					>
						<Copy class="size-[18px]" aria-hidden="true" />
					</span>
				{/if}
			</button>
		</div>
	{/if}
</div>
