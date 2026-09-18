<script lang="ts">
	import { resolve } from '$app/paths';
	import { updateRecipe, type RecipeInput } from '$lib/api/recipes';
	import RecipeForm from '$lib/components/editor/RecipeForm.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const recipe = $derived(data.recipe);

	function save(input: RecipeInput) {
		return updateRecipe(recipe.id, input);
	}
</script>

<svelte:head>
	<title>{recipe.title} · {m.editor_title_edit()}</title>
</svelte:head>

<!-- `RecipeForm` snapshots `initial` once, so editing a different recipe
     without leaving the route (a link from one editor to another) has to
     give it a fresh instance rather than a new prop. -->
{#key recipe.id}
	<RecipeForm
		initial={recipe}
		existing={recipe}
		heading={m.editor_title_edit()}
		cancelHref={resolve('/recipes/[slug]', { slug: recipe.slug })}
		{save}
	/>
{/key}
