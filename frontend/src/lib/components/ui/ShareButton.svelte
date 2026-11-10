<script lang="ts">
	import Share2 from '@lucide/svelte/icons/share-2';
	import IconButton from './IconButton.svelte';

	// The round share button of a page's head (a recipe, a list), so the
	// glyph sits alike everywhere.
	let {
		label,
		onclick,
		class: className = ''
	}: {
		label: string;
		onclick: (event: MouseEvent) => void;
		class?: string;
	} = $props();
</script>

<IconButton {label} {onclick} class={className}>
	<!-- Centered by its box, the glyph still leans right: two of its three
	     dots sit there, its ink 1.3px right of the middle at 20px. A pixel
	     left centers it to the eye. -->
	<Share2 class="size-5 -translate-x-px" aria-hidden="true" />
</IconButton>
