<script lang="ts">
	import type { Recipe } from '$lib/api/recipes';
	import { m } from '$lib/paraglide/messages';
	import { hasBeenEdited } from '$lib/recipe/authorship';
	import { formatDate } from '$lib/recipe/format';

	let {
		recipe
	}: {
		recipe: Pick<Recipe, 'createdBy' | 'createdAt' | 'updatedBy' | 'updatedAt'>;
	} = $props();

	const edited = $derived(hasBeenEdited(recipe));
</script>

<footer class="mt-10 border-t border-border pt-6 text-caption text-text-muted">
	<p>
		{m.detail_created_by({
			user: recipe.createdBy.displayName,
			date: formatDate(recipe.createdAt)
		})}
	</p>
	{#if edited}
		<p class="mt-1">
			{m.detail_updated_by({
				user: recipe.updatedBy.displayName,
				date: formatDate(recipe.updatedAt)
			})}
		</p>
	{/if}
</footer>
