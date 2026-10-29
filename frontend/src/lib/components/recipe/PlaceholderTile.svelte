<script lang="ts">
	import { initialFor, tintFor } from '#lib/recipe/placeholder.js';

	let {
		id,
		title,
		size = 'card',
		class: className = ''
	}: {
		id: string;
		title: string;
		/** Initial glyph size: 72px for overview cards; a larger size reserved
		 * for the recipe detail page's cover placeholder (Task B4). */
		size?: 'card' | 'detail';
		/** Sizing/shape for the tile itself (e.g. `aspect-square rounded-lg`)
		 * - this component only fills its container. */
		class?: string;
	} = $props();

	const SIZE_CLASSES = {
		card: 'text-[72px]',
		detail: 'text-[120px]'
	} as const;

	const tint = $derived(tintFor(id));
	const initial = $derived(initialFor(title));
</script>

<div aria-hidden="true" class="flex size-full items-center justify-center {tint} {className}">
	<span class="font-display font-medium text-primary italic {SIZE_CLASSES[size]}">
		{initial}
	</span>
</div>
