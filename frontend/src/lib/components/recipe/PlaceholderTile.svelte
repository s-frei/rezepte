<script lang="ts">
	import { tintFor, type Tint } from '$lib/recipe/placeholder';

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

	const TINT_CLASSES: Record<Tint, string> = {
		'tint-1': 'bg-tint-1',
		'tint-2': 'bg-tint-2',
		'tint-3': 'bg-tint-3'
	};

	const SIZE_CLASSES = {
		card: 'text-[72px]',
		detail: 'text-[120px]'
	} as const;

	const tint = $derived(tintFor(id));
	const initial = $derived(title.trim().charAt(0).toUpperCase() || '?');
</script>

<div
	aria-hidden="true"
	class="flex size-full items-center justify-center {TINT_CLASSES[tint]} {className}"
>
	<span class="font-display font-medium text-primary italic {SIZE_CLASSES[size]}">
		{initial}
	</span>
</div>
