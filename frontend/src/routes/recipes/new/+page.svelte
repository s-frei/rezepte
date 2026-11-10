<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { afterNavigate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import {
		createRecipe,
		emptyInput,
		updateRecipe,
		uploadImage,
		type Recipe,
		type RecipeInput
	} from '#lib/api/recipes.js';
	import RecipeForm from '#lib/components/editor/RecipeForm.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { takeDraft, type ImportedDraft } from '#lib/recipe/draft.js';
	import { resolvePhotos, type QueuedPhoto } from '#lib/recipe/step-photos.js';

	// `?title=` prefills the title and nothing else, for "Als neues Rezept" on
	// the overview's no-results state: someone searched for a dish, found
	// none, and is about to write it down under that name. It stays a purely
	// client-side hand-over - the recipe is created on "Speichern" like any
	// other, so no API or contract is involved in carrying the term across.
	//
	// Read once, like `RecipeForm` reads `initial` once: these are starting
	// values, and re-seeding them from a later URL would throw away whatever
	// has been typed since.
	// An import from the dialog arrives the same way, once: a reload finds
	// nothing and opens the empty editor. The draft is spread over
	// `emptyInput()` because the server leaves out what it has no value for.
	function startFrom(draft: ImportedDraft | null): RecipeInput {
		return untrack(() =>
			draft
				? { ...emptyInput(), ...draft.draft.recipe }
				: { ...emptyInput(), title: page.url.searchParams.get('title')?.trim() ?? '' }
		);
	}

	function announce(draft: ImportedDraft | null) {
		if (draft?.photoFailed) toast.error(m.import_photo_failed());
		if (draft?.draft.truncated) toast(m.import_truncated());
	}

	const first = takeDraft();
	let imported = $state.raw(first);
	let initial = $state.raw(startFrom(first));
	onMount(() => announce(first));

	// An import started while this page is open (palette, top bar) arrives
	// by a navigation to the same URL, which keeps the page: take it then and
	// start the editor over from it. The leave guard has had its say by now.
	afterNavigate(() => {
		const next = takeDraft();
		if (!next) return;
		imported = next;
		initial = startFrom(next);
		announce(next);
	});

	// Set when the links could not be set: the save opens the new recipe's editor.
	let relink = false;
	const savedHref = (recipe: Recipe) =>
		relink
			? resolve('/recipes/[slug]/edit', { slug: recipe.slug })
			: resolve('/recipes/[slug]', { slug: recipe.slug });

	/**
	 * Creates the recipe, then uploads the queued images one by one. A failed
	 * upload is reported but never undoes the recipe - the user lands on the
	 * detail page and can retry from the editor.
	 *
	 * A step's photo link names a queued photo by its key, which only the
	 * upload turns into an id: the recipe is created without the links, and
	 * once the photos are up one update sets them, a link to a photo whose
	 * upload failed left out - and then the author lands in the editor too.
	 */
	async function save(input: RecipeInput, pending: QueuedPhoto[]) {
		relink = false;
		const withPhotos = (ids: Map<string, string>) =>
			input.steps.map((step) => resolvePhotos(step, ids));
		let recipe = await createRecipe({ ...input, steps: withPhotos(new Map()) });
		const uploaded: [string, string][] = [];
		const failed: string[] = [];
		for (const { key, file } of pending) {
			try {
				uploaded.push([key, (await uploadImage(recipe.id, file)).id]);
			} catch {
				failed.push(key);
			}
		}
		const ids = new Map(uploaded);
		const linked = input.steps.flatMap((step) => step.photos ?? []);
		if (linked.some((photo) => failed.includes(photo.imageId))) {
			relink = true;
			toast.error(m.editor_photo_links_failed());
		}
		if (linked.some((photo) => ids.has(photo.imageId))) {
			// The recipe stands either way: thrown, the author would stay here
			// and the next Save would create it, photos and all, a second time.
			try {
				recipe = await updateRecipe(recipe.id, { ...input, steps: withPhotos(ids) });
			} catch {
				// Its editor, not its page: the author relinks there and saves.
				if (!relink) toast.error(m.editor_photo_links_failed());
				relink = true;
			}
		}
		if (failed.length === 1) {
			toast.error(m.images_upload_error());
		} else if (failed.length > 1) {
			toast.error(m.images_upload_error_count({ count: failed.length }));
		}
		return recipe;
	}
</script>

<svelte:head>
	<title>{m.overview_new_recipe()} · {m.app_name()}</title>
</svelte:head>

{#key initial}
	<RecipeForm
		{initial}
		imported={imported ?? undefined}
		heading={m.overview_new_recipe()}
		cancelHref={resolve('/')}
		{save}
		{savedHref}
	/>
{/key}
