<script lang="ts">
	import Lock from 'lucide-svelte/icons/lock';
	import type { Recipe } from '$lib/api/recipes';
	import { session } from '$lib/auth.svelte';
	import { m } from '$lib/paraglide/messages';
	import { lockNotice } from '$lib/recipe/access';
	import { hasBeenEdited } from '$lib/recipe/authorship';
	import { formatDate } from '$lib/recipe/format';

	let {
		recipe
	}: {
		recipe: Pick<Recipe, 'createdBy' | 'createdAt' | 'updatedBy' | 'updatedAt' | 'locked'>;
	} = $props();

	const edited = $derived(hasBeenEdited(recipe));
	const notice = $derived(lockNotice(recipe, session.user?.id));
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
	{#if notice}
		<p class="mt-1 flex items-center gap-1.5">
			<Lock class="size-3.5 shrink-0" aria-hidden="true" />
			{notice}
		</p>
	{/if}
</footer>
