<script lang="ts">
	import Clock from '@lucide/svelte/icons/clock';
	import Users from '@lucide/svelte/icons/users';
	import { imageUrl, type RecipeContent } from '$lib/api/recipes';
	import Lockup from '$lib/components/brand/Lockup.svelte';
	import { SOURCE_URL } from '$lib/docs';
	import { formatMinutes, formatServings } from '$lib/recipe/format';
	import { formatQuantityFor } from '$lib/recipe/scale';
	import { m } from '$lib/paraglide/messages';
	import PlaceholderTile from './PlaceholderTile.svelte';
	import StepList from './StepList.svelte';

	// The recipe card a member passes on as a picture (see `ShareSheet`).
	// Laid out at the phone page's 420px with the page's own type sizes, so
	// the picture reads like the page however far a messenger scales it.
	// Nothing here is interactive: no checkboxes, no stepper, no links.
	let {
		recipe,
		servings,
		attribution
	}: {
		recipe: RecipeContent;
		/** The servings the user chose; quantities are scaled to them. */
		servings: number;
		/** The household's "name Rezepte when sharing" setting: the footer. */
		attribution: boolean;
	} = $props();

	const coverId = $derived(recipe.coverImageId ?? recipe.images[0]?.id ?? null);
	let coverFailed = $state(false);
	const source = SOURCE_URL.replace(/^https?:\/\//, '');
	const pill =
		'inline-flex h-8 items-center gap-1.5 rounded-pill bg-surface px-3 text-caption font-medium shadow-card';
</script>

<div class="w-[420px] bg-background text-text">
	{#if coverId && !coverFailed}
		<img
			src={imageUrl(recipe.id, coverId, 'detail')}
			alt=""
			class="block h-[290px] w-full object-cover"
			onerror={() => (coverFailed = true)}
		/>
	{:else}
		<PlaceholderTile id={recipe.id} title={recipe.title} size="detail" class="h-[290px] w-full" />
	{/if}
	<div class="px-6 pt-6">
		<h1 class="font-display text-display-md font-medium wrap-break-word hyphens-auto">
			{recipe.title}
		</h1>
		{#if recipe.description}
			<p class="mt-2.5 text-body text-text-muted">{recipe.description}</p>
		{/if}
		<div class="mt-4 flex flex-wrap gap-1.5">
			<span class={pill}>
				<Users class="size-3.5 text-primary" aria-hidden="true" />
				{formatServings(servings)}
			</span>
			{#if recipe.prepMinutes !== null}
				<span class={pill}>
					<Clock class="size-3.5 text-primary" aria-hidden="true" />
					{formatMinutes(recipe.prepMinutes)}
					{m.recipe_prep_time()}
				</span>
			{/if}
			{#if recipe.cookMinutes !== null}
				<span class={pill}>
					<Clock class="size-3.5 text-primary" aria-hidden="true" />
					{formatMinutes(recipe.cookMinutes)}
					{m.recipe_cook_time()}
				</span>
			{/if}
		</div>

		<h2 class="mt-7 mb-3 font-display text-heading font-medium">{m.recipe_ingredients()}</h2>
		<div class="rounded-2xl bg-surface px-5 pt-[18px] pb-2">
			{#each recipe.ingredientGroups as group, groupIndex (groupIndex)}
				{#if group.name}
					<h3 class="mt-4 mb-1 font-display text-[16px] font-medium text-primary italic first:mt-0">
						{group.name}
					</h3>
				{/if}
				<ul>
					{#each group.ingredients as ingredient, ingredientIndex (ingredientIndex)}
						<li
							class="grid grid-cols-[72px_1fr] gap-2 border-b border-dashed border-border py-2 last:border-b-0"
						>
							<span class="text-body-sm font-semibold tabular-nums">
								{formatQuantityFor(ingredient.quantity, recipe.servings, servings)}
								{ingredient.unit ?? ''}
							</span>
							<span class="text-body">
								{ingredient.name}
								{#if ingredient.note}
									<span class="ml-1 text-caption text-text-muted">({ingredient.note})</span>
								{/if}
							</span>
						</li>
					{/each}
				</ul>
			{/each}
		</div>

		<h2 class="mt-7 mb-3.5 font-display text-heading font-medium">{m.recipe_steps()}</h2>
		<StepList
			steps={recipe.steps}
			groups={recipe.ingredientGroups}
			{servings}
			baseServings={recipe.servings}
		/>
	</div>

	{#if attribution}
		<!-- The ingredient list's perforation, so the credit reads as a stub
		     torn off the card rather than part of the recipe. -->
		<footer class="relative mt-8 flex items-center justify-between gap-4 px-6 pt-[22px] pb-[26px]">
			<span
				class="absolute inset-x-[18px] top-0 h-1 bg-[radial-gradient(circle,var(--color-handle)_1.6px,transparent_2px)] bg-[length:9px_4px] bg-repeat-x"
				aria-hidden="true"
			></span>
			<Lockup variant="horizontal" class="h-10" />
			<p class="text-right text-caption text-text-muted">
				{m.share_card_footer()}<br />
				<span class="font-semibold text-primary">{source}</span>
			</p>
		</footer>
	{:else}
		<div class="h-8"></div>
	{/if}
</div>
