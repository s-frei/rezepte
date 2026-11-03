<script lang="ts">
	import { imageUrl } from '#lib/api/recipes.js';
	import type { ShoppingEntry } from '#lib/api/shopping.js';
	import { initialFor, tintFor } from '#lib/recipe/placeholder.js';

	let { entry, class: className = '' }: { entry: ShoppingEntry; class?: string } = $props();
</script>

<!-- The recipe's thumbnail, or its tinted initial like a card without photos (and once the recipe is gone). -->
{#if entry.recipeId && entry.coverImageId}
	<img
		src={imageUrl(entry.recipeId, entry.coverImageId, 'thumb')}
		alt=""
		class="shrink-0 rounded-lg object-cover {className}"
	/>
{:else}
	<span
		aria-hidden="true"
		class="flex shrink-0 items-center justify-center rounded-lg font-display font-medium text-primary italic {tintFor(
			entry.recipeId ?? entry.id
		)} {className}">{initialFor(entry.title)}</span
	>
{/if}
