<script lang="ts">
	import type { Tag } from '$lib/api/recipes';
	import TagChip from '$lib/components/ui/TagChip.svelte';

	let {
		tags,
		active,
		ontoggle
	}: {
		/** All tags currently in use, from `listTags()`. */
		tags: Tag[];
		/** Currently selected tag names. */
		active: string[];
		ontoggle: (name: string) => void;
	} = $props();
</script>

{#if tags.length > 0}
	<div class="flex gap-2 overflow-x-auto">
		{#each tags as tag (tag.name)}
			<TagChip
				label={tag.name}
				count={tag.count}
				active={active.includes(tag.name)}
				onclick={() => ontoggle(tag.name)}
			/>
		{/each}
	</div>
{/if}
