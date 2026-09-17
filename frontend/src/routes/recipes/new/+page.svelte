<script lang="ts">
	import { resolve } from '$app/paths';
	import { createRecipe, emptyInput } from '$lib/api/recipes';
	import RecipeForm from '$lib/components/editor/RecipeForm.svelte';
	import { m } from '$lib/paraglide/messages';

	// `?import=` is reserved for a later phase (URL import). It is
	// deliberately ignored here so a link carrying it still opens a blank
	// editor instead of 404-ing once the flow lands.
	const initial = emptyInput();
</script>

<svelte:head>
	<title>{m.overview_new_recipe()} · {m.app_name()}</title>
</svelte:head>

<RecipeForm
	{initial}
	heading={m.overview_new_recipe()}
	cancelHref={resolve('/')}
	save={createRecipe}
/>
