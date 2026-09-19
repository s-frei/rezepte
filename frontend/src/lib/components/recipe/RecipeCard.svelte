<script lang="ts">
	import { resolve } from '$app/paths';
	import { imageUrl, type RecipeCard as RecipeCardData } from '$lib/api/recipes';
	import { formatMinutes } from '$lib/recipe/format';
	import PlaceholderTile from './PlaceholderTile.svelte';

	let { recipe }: { recipe: RecipeCardData } = $props();

	const time = $derived(formatMinutes(recipe.totalMinutes));
</script>

<a
	href={resolve('/recipes/[slug]', { slug: recipe.slug })}
	class="block overflow-hidden rounded-xl bg-surface shadow-card transition hover:brightness-[0.98] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:rounded-2xl"
>
	<div class="aspect-square p-1.5 md:p-2">
		{#if recipe.coverImageId}
			<img
				src={imageUrl(recipe.id, recipe.coverImageId, 'thumb')}
				alt=""
				loading="lazy"
				decoding="async"
				class="size-full rounded-lg object-cover"
			/>
		{:else}
			<PlaceholderTile id={recipe.id} title={recipe.title} class="overflow-hidden rounded-lg" />
		{/if}
	</div>
	<div class="px-4 pt-2 pb-[18px]">
		<!-- The title is what you recognise the card by, so it wraps rather than
		     cutting off ("Schweineschnitzel mit Bratka…"). Two line heights are
		     reserved either way, so the time pills of a row stay on one line.
		     A phone card leaves 124px of text, which German compounds outrun on
		     their own - so `hyphens-auto` (the document is `lang="de"`) lets
		     them break instead of running out of the card unseen. Two lines on
		     a phone - of twelve seeded titles exactly two need a third there,
		     and granting it would raise every row for their sake - but three on
		     desktop, where 19px type pushes three titles past two lines and a
		     four-column grid has the height to spare. -->
		<h3
			class="line-clamp-2 min-h-[2lh] font-display text-[16px] font-medium hyphens-auto md:line-clamp-3 md:text-card"
		>
			{recipe.title}
		</h3>
		{#if recipe.tags.length > 0}
			<p class="mt-1 truncate text-caption text-text-muted">{recipe.tags.join(', ')}</p>
		{/if}
		{#if time}
			<span
				class="mt-2 inline-flex h-5 items-center rounded-pill bg-accent px-2 text-micro font-semibold text-accent-foreground"
			>
				{time}
			</span>
		{/if}
	</div>
</a>
