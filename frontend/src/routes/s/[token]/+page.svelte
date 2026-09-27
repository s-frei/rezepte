<script lang="ts">
	import { publicImageUrl } from '$lib/api/public';
	import type { ImageVariant, RecipeContent } from '$lib/api/recipes';
	import Lockup from '$lib/components/brand/Lockup.svelte';
	import RecipeView from '$lib/components/recipe/RecipeView.svelte';
	import Lightbox from '$lib/components/ui/Lightbox.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// `RecipeContent` from the public response plus a client-side-only id
	// (servings memory, gallery keys - `RecipeView` never sends it to the
	// server). The real recipe id is never in this page's reach: the
	// backend's PublicRecipe carries no id at all. Its images carry no
	// `position` either (nothing here reorders them) - the array is already
	// in gallery order, so the index stands in for it.
	const recipe = $derived<RecipeContent | null>(
		data.recipe && {
			...data.recipe,
			id: `share:${data.token}`,
			images: data.recipe.images.map((image, position) => ({ ...image, position }))
		}
	);

	function imageSrc(imageId: string, variant: ImageVariant): string {
		return publicImageUrl(data.token, imageId, variant);
	}

	let lightboxOpen = $state(false);
	let lightboxIndex = $state(0);

	function openLightbox(index: number) {
		lightboxIndex = index;
		lightboxOpen = true;
	}
</script>

<svelte:head>
	<title>{recipe ? `${recipe.title} · ${m.app_name()}` : m.app_name()}</title>
</svelte:head>

<div class="mx-auto flex min-h-dvh max-w-[1280px] flex-col px-5 py-8 md:px-8 md:py-10">
	<header class="mb-8">
		<Lockup variant="compact" label={m.app_name()} class="h-7" />
	</header>

	<main class="flex-1">
		{#if recipe}
			<RecipeView {recipe} {imageSrc} onopenimage={openLightbox} />
		{:else}
			<p class="text-body text-text-muted">{m.public_share_unavailable()}</p>
		{/if}
	</main>

	<footer class="mt-10 border-t border-border pt-6 text-caption text-text-muted">
		{m.public_share_footer()}
	</footer>
</div>

{#if recipe}
	<Lightbox
		bind:open={lightboxOpen}
		bind:index={lightboxIndex}
		recipeId={recipe.id}
		title={recipe.title}
		images={recipe.images}
		src={imageSrc}
	/>
{/if}
