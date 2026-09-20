<script lang="ts">
	import { untrack } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import { createRecipe, emptyInput, uploadImage, type RecipeInput } from '$lib/api/recipes';
	import RecipeForm from '$lib/components/editor/RecipeForm.svelte';
	import { m } from '$lib/paraglide/messages';

	// `?title=` prefills the title and nothing else, for "Als neues Rezept" on
	// the overview's no-results state: someone searched for a dish, found
	// none, and is about to write it down under that name. It stays a purely
	// client-side hand-over - the recipe is created on "Speichern" like any
	// other, so no API or contract is involved in carrying the term across.
	//
	// Read once, like `RecipeForm` reads `initial` once: these are starting
	// values, and re-seeding them from a later URL would throw away whatever
	// has been typed since.
	const initial = untrack(() => ({
		...emptyInput(),
		title: page.url.searchParams.get('title')?.trim() ?? ''
	}));

	/**
	 * Creates the recipe, then uploads the queued images one by one. A failed
	 * upload is reported but never undoes the recipe - the user lands on the
	 * detail page and can retry from the editor.
	 */
	async function save(input: RecipeInput, pendingFiles: File[]) {
		const recipe = await createRecipe(input);
		let failed = 0;
		for (const file of pendingFiles) {
			try {
				await uploadImage(recipe.id, file);
			} catch {
				failed += 1;
			}
		}
		if (failed === 1) {
			toast.error(m.images_upload_error());
		} else if (failed > 1) {
			toast.error(m.images_upload_error_count({ count: failed }));
		}
		return recipe;
	}
</script>

<svelte:head>
	<title>{m.overview_new_recipe()} · {m.app_name()}</title>
</svelte:head>

<RecipeForm {initial} heading={m.overview_new_recipe()} cancelHref={resolve('/')} {save} />
