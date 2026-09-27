<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ImageVariant, RecipeContent } from '$lib/api/recipes';
	import TagChip from '$lib/components/ui/TagChip.svelte';
	import ImageGallery from './ImageGallery.svelte';
	import IngredientList from './IngredientList.svelte';
	import MetaPills from './MetaPills.svelte';
	import PlaceholderTile from './PlaceholderTile.svelte';
	import SourceCredit from './SourceCredit.svelte';
	import ServingsStepper from './ServingsStepper.svelte';
	import StepList from './StepList.svelte';
	import { createServings } from '$lib/recipe/servings.svelte';
	import { m } from '$lib/paraglide/messages';

	let {
		recipe,
		imageSrc,
		tagHref,
		onopenimage,
		byline,
		kicker,
		belowMeta,
		footer
	}: {
		recipe: RecipeContent;
		/** Forwarded to `ImageGallery`; defaults there to the signed-in `imageUrl` route. */
		imageSrc?: (imageId: string, variant: ImageVariant) => string;
		/** A tag's destination. Omitted renders plain, non-interactive chips. */
		tagHref?: (tag: string) => string;
		/** Called with the index of the picture the caller tapped/clicked, for a lightbox it owns. */
		onopenimage: (index: number) => void;
		/** The line under the title, before the description - the detail page
		 * sets its favorite star, tasty heart and "tasty" people here, so the
		 * title keeps the whole width. */
		byline?: Snippet;
		/** Rendered at the end of the tag row above the title, which it opens
		 * even for a recipe without tags - the detail page's public-link
		 * marker lives here: a status of the recipe as a whole, which wraps
		 * with the tags instead of crowding the title. */
		kicker?: Snippet;
		/** Rendered after the meta pills, mobile-only in practice - the detail
		 * page's sticky "start cooking" call to action lives here: it needs the
		 * slug-based cook route, which is not part of `RecipeContent`. */
		belowMeta?: Snippet;
		/** Rendered after the ingredients/steps - the detail page puts `RecipeColophon` here. */
		footer?: Snippet;
	} = $props();

	// One store per recipe: `$derived` re-creates it when `recipe` changes (a
	// caller reusing this component for another recipe), reading that
	// recipe's stored choice. Mutations go through `servings.set()`, not this
	// binding.
	const servings = $derived(createServings(recipe.id, recipe.servings));
</script>

<div class="mb-6 h-[260px] overflow-hidden rounded-3xl bg-surface p-3 md:hidden">
	{#if recipe.images.length > 0}
		<ImageGallery
			recipeId={recipe.id}
			title={recipe.title}
			images={recipe.images}
			coverId={recipe.coverImageId}
			layout="mobile"
			onopen={onopenimage}
			src={imageSrc}
		/>
	{:else}
		<PlaceholderTile
			id={recipe.id}
			title={recipe.title}
			size="detail"
			class="size-full rounded-2xl"
		/>
	{/if}
</div>

<!--
	The cover column is `minmax(0,420px)`, not a flat `420px`: a `1fr`
	track cannot shrink past its own min-content, and here that floor
	is the longest word of the title at 52px - 335px for "Königsberger".
	With a rigid 420px beside it the row demanded 795px, which is more
	than this card has between 768px (where the two columns appear) and
	about 858px, and the cover hung over the edge of the page. Letting
	the cover give way keeps the title's floor intact; above 858px the
	cover still takes its full 420px and nothing about this row changes.

	The text column keeps its plain `1fr` on purpose. `minmax(0,1fr)`
	there would take the floor out from under the title instead, and
	the word would leave its column rather than the cover leaving the
	page - the same overflow, one step further in.
-->
<div class="md:grid md:grid-cols-[1fr_minmax(0,420px)] md:items-start md:gap-10">
	<div>
		{#if recipe.tags.length > 0 || kicker}
			<div class="flex flex-wrap items-center gap-2">
				{#each recipe.tags as tag (tag)}
					{#if tagHref}
						<!-- `tagHref` is a caller-built address, not a route id this
						     component could resolve() itself. -->
						<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
						<a href={tagHref(tag)}><TagChip label={tag} /></a>
					{:else}
						<TagChip label={tag} />
					{/if}
				{/each}
				{@render kicker?.()}
			</div>
		{/if}
		<!-- The title has the column to itself. A long compound that still
		     does not fit a 320px phone breaks, as in cook mode, rather than
		     push the page sideways. -->
		<h1
			class="mt-3 font-display text-display-md font-medium wrap-break-word hyphens-auto [hyphenate-limit-chars:12_4_4] md:mt-4 md:text-display-xl"
		>
			{recipe.title}
		</h1>
		{#if byline}
			<div class="mt-3 flex flex-wrap items-center gap-x-2 gap-y-2">
				{@render byline()}
			</div>
		{/if}
		{#if recipe.description}
			<p class="mt-3 text-body-lg text-text-muted">{recipe.description}</p>
		{/if}
		<div class="mt-2 empty:hidden">
			<SourceCredit name={recipe.sourceName} url={recipe.sourceUrl} />
		</div>
		<div class="mt-5">
			<MetaPills prepMinutes={recipe.prepMinutes} cookMinutes={recipe.cookMinutes}>
				<ServingsStepper value={servings.value} onchange={(next) => servings.set(next)} />
				{#if servings.scaled}
					<button
						type="button"
						onclick={() => servings.reset()}
						class="text-body-sm font-medium text-primary underline-offset-4 transition hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
					>
						{m.servings_reset()}
					</button>
				{/if}
			</MetaPills>
		</div>
		{#if belowMeta}
			{@render belowMeta()}
		{/if}
	</div>
	<div class="hidden md:block">
		{#if recipe.images.length > 0}
			<ImageGallery
				recipeId={recipe.id}
				title={recipe.title}
				images={recipe.images}
				coverId={recipe.coverImageId}
				layout="desktop"
				onopen={onopenimage}
				src={imageSrc}
			/>
		{:else}
			<PlaceholderTile
				id={recipe.id}
				title={recipe.title}
				size="detail"
				class="aspect-[4/3] rounded-3xl shadow-cover"
			/>
		{/if}
	</div>
</div>

<div class="mt-8 grid gap-8 md:mt-10 md:grid-cols-[400px_1fr] md:gap-10">
	<section>
		<h2 class="mb-4 font-display text-heading font-medium">{m.recipe_ingredients()}</h2>
		<IngredientList
			recipeId={recipe.id}
			groups={recipe.ingredientGroups}
			servings={servings.value}
			baseServings={servings.base}
		/>
	</section>
	<section>
		<h2 class="mb-5 font-display text-heading font-medium">{m.recipe_steps()}</h2>
		<StepList
			steps={recipe.steps}
			groups={recipe.ingredientGroups}
			servings={servings.value}
			baseServings={servings.base}
		/>
	</section>
</div>

{#if footer}
	{@render footer()}
{/if}
