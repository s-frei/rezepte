<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';

	let {
		progress,
		size
	}: {
		/** The upload's share done, 0 to 1. */
		progress: number;
		/** Its width and height in px. */
		size: number;
	} = $props();
</script>

<!-- An upload's progress, drawn over the photo going up. The ring's radius
     is 40% of `size` and its stroke 4px, at any size. -->
<svg
	viewBox="0 0 40 40"
	width={size}
	height={size}
	stroke-width={160 / size}
	role="progressbar"
	aria-label={m.images_uploading()}
	aria-valuemin={0}
	aria-valuemax={100}
	aria-valuenow={Math.round(progress * 100)}
	class="-rotate-90 fill-none"
>
	<circle cx="20" cy="20" r="16" class="stroke-lightbox-foreground/40" />
	<circle
		cx="20"
		cy="20"
		r="16"
		pathLength="1"
		stroke-dasharray="1"
		stroke-dashoffset={1 - progress}
		stroke-linecap="round"
		class="stroke-lightbox-foreground transition-[stroke-dashoffset] duration-200 motion-reduce:transition-none"
	/>
</svg>
