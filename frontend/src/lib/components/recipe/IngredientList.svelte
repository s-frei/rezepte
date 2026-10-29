<script lang="ts">
	import { Checkbox } from 'bits-ui';
	import { prefersReducedMotion } from 'svelte/motion';
	import { backOut } from 'svelte/easing';
	import { fade, scale } from 'svelte/transition';
	import Check from '@lucide/svelte/icons/check';
	import type { IngredientGroup } from '#lib/api/recipes.js';
	import { ingredientKey } from '#lib/recipe/checked.js';
	import { isChecked, toggle } from '#lib/recipe/checked.svelte.js';
	import { formatFactor, formatServings } from '#lib/recipe/format.js';
	import { MIN_ROWS, splitColumns, type ColumnPart } from '#lib/recipe/ingredient-columns.js';
	import { shoppingList } from '#lib/recipe/ingredient-text.js';
	import { formatQuantityFor } from '#lib/recipe/scale.js';
	import { copyText } from '#lib/recipe/share.svelte.js';
	import CopyIcon from '#lib/components/icons/CopyIcon.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let {
		recipeId,
		groups,
		servings,
		baseServings,
		copyable = false,
		columns: split = false
	}: {
		recipeId: string;
		groups: IngredientGroup[];
		/** The servings the user chose (1-99). */
		servings: number;
		/** The recipe's stored servings - what every quantity is written for. */
		baseServings: number;
		/** Ends the card in a tear-off shopping list. */
		copyable?: boolean;
		/** Two columns on a tablet, where the stacked recipe spans the page. */
		columns?: boolean;
	} = $props();

	const uid = $props.id();
	let slipIcon = $state<CopyIcon>();
	const scaled = $derived(servings !== baseServings);
	// A check springs in and a recomputed amount answers the stepper with a
	// brief grow; neither moves under reduced motion. Taking a check back has
	// no out transition, so it snaps.
	const grow = $derived(prefersReducedMotion.current ? 0 : 150);
	const columns = $derived(splitColumns(groups, split ? MIN_ROWS : Infinity));
	const twoUp = $derived(columns[1].length > 0);
	const columnGrid = $derived(
		`col-span-full grid grid-cols-subgrid content-start ${
			twoUp
				? 'min-[600px]:max-[840px]:col-span-1 min-[600px]:max-[840px]:grid-cols-[22px_max-content_minmax(0,1fr)] min-[600px]:max-[840px]:gap-x-2'
				: ''
		}`
	);
	// A divider under every row but a group's last; a group broken across the
	// columns keeps it under the left column's last row only while they stack.
	const rowRule = (part: ColumnPart, index: number) =>
		index === groups[part.group].ingredients.length - 1
			? ''
			: index === part.to - 1
				? 'border-b min-[600px]:max-[840px]:border-b-0'
				: 'border-b';
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
		<!-- A group heading starts its column without a top margin, so the hint keeps the gap. -->
		<p
			class="text-caption font-medium text-text-muted {groups[0]?.name ? 'mb-5' : 'mb-3'}"
			in:fade={{ duration: 150 }}
		>
			{m.servings_scaled_hint({
				servings: formatServings(servings),
				factor: formatFactor(baseServings, servings)
			})}
		</p>
	{/if}
	{#snippet column(parts: ColumnPart[], right: boolean)}
		{#each parts as part (part.group)}
			{@const group = groups[part.group]}
			{@const groupIndex = part.group}
			{#if part.heading}
				<h3
					class="col-span-full mt-5 mb-2 font-display text-[16px] font-medium text-primary italic {right
						? 'min-[600px]:max-[840px]:first:mt-0'
						: 'first:mt-0'}"
				>
					{group.name}
				</h3>
			{/if}
			<ul class="col-span-full grid grid-cols-subgrid">
				{#each group.ingredients.slice(part.from, part.to) as ingredient, i (i)}
					{@const ingredientIndex = part.from + i}
					{@const key = ingredientKey(recipeId, groupIndex, ingredientIndex)}
					{@const rowChecked = isChecked(key)}
					{@const amountInk = rowChecked
						? 'text-text-muted transition-colors duration-220 ease-glide'
						: 'text-primary'}
					{@const amount = formatQuantityFor(ingredient.quantity, baseServings, servings)}
					<!-- The ink fades to muted with the strike and snaps back without it.
						 The number gets the shared column; the unit leads the name, so a
					     long unit wraps with the text instead of widening every row. -->
					<li
						class="col-span-full grid min-h-11 grid-cols-subgrid content-center items-baseline border-dashed border-border py-2 md:min-h-10 {rowRule(
							part,
							ingredientIndex
						)}"
					>
						<!-- On the first line, beside the amount, however far the name wraps. -->
						<Checkbox.Root
							checked={rowChecked}
							onCheckedChange={() => toggle(key)}
							aria-label={ingredient.name}
							class="mt-[calc((1lh_-_20px)/2)] flex size-5 items-center justify-center self-start rounded-full border-[1.5px] border-handle text-body transition data-[state=checked]:border-primary data-[state=checked]:bg-primary"
						>
							{#snippet children({ checked: isRowChecked })}
								{#if isRowChecked}
									<span class="flex" in:scale={{ start: 0.4, duration: grow, easing: backOut }}>
										<Check class="size-3 text-primary-foreground" aria-hidden="true" />
									</span>
								{/if}
							{/snippet}
						</Checkbox.Root>
						<!-- Keyed on the text, so only an amount the servings changed answers;
							 it starts at 60% ink, as an empty frame would read as a flicker. -->
						{#key amount}
							<span
								class="origin-right text-right text-body font-semibold tabular-nums {amountInk}"
								in:scale={{ start: 0.9, opacity: 0.6, duration: grow }}
							>
								<span class="strike" data-struck={rowChecked || undefined}>{amount}</span>
							</span>
						{/key}
						<!-- German compounds ("Rinderhackfleischbällchen") outrun a 320px row, so
						     they hyphenate; the limit keeps "Muskatnuss" whole on the next line. -->
						<span class="text-body break-words hyphens-auto [hyphenate-limit-chars:12_6_6]">
							<!-- One struck run, so the space between unit and name is struck too. -->
							<span
								class="strike {rowChecked ? 'text-text-muted' : 'text-text'}"
								data-struck={rowChecked || undefined}
							>
								{#if ingredient.unit}
									<span class="font-semibold {amountInk}">{ingredient.unit}</span>
								{/if}
								{ingredient.name}
							</span>
							{#if ingredient.note}
								<span class="text-caption text-text-muted">({ingredient.note})</span>
							{/if}
						</span>
					</li>
				{/each}
			</ul>
		{/each}
	{/snippet}
	<!-- Two columns only on a tablet, where the stacked list spans the full card;
	     the split lives in splitColumns, so a column never ends on a stray divider.
	     Stacked, both columns share this grid, so every number ends on one edge;
	     side by side, each column lines up its own. -->
	<div
		class="grid grid-cols-[22px_max-content_minmax(0,1fr)] content-start gap-x-2 {twoUp
			? 'min-[600px]:max-[840px]:grid-cols-2 min-[600px]:max-[840px]:gap-x-5'
			: ''}"
	>
		<div class="{columnGrid} min-w-0">{@render column(columns[0], false)}</div>
		{#if twoUp}
			<div
				class="{columnGrid} min-w-0 border-dashed border-border min-[600px]:max-[840px]:border-l min-[600px]:max-[840px]:pl-5"
			>
				{@render column(columns[1], true)}
			</div>
		{/if}
	</div>
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
				onclick={async () => {
					if (
						await copyText(
							shoppingList(groups, baseServings, servings, checkedAt),
							m.ingredients_slip_copied()
						)
					) {
						slipIcon?.play();
					}
				}}
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
						<CopyIcon bind:this={slipIcon} class="size-[18px]" />
					</span>
				{/if}
			</button>
		</div>
	{/if}
</div>
