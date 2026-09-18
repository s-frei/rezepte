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
	<!-- Phones scroll the row sideways; on desktop there is room to wrap, and a
	     filter you cannot see is a filter you will not use. -->
	<div class="flex gap-2 overflow-x-auto md:flex-wrap md:overflow-x-visible">
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
