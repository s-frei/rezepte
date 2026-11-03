<script lang="ts">
	import type { Snippet } from 'svelte';
	import { m } from '#lib/paraglide/messages.js';

	let {
		photos,
		onopen,
		onmeasure,
		children
	}: {
		/** One to four, in the order shown; `key` is also each cell's `data-sortable-key`. */
		photos: { key: string; src: string; width: number; height: number }[];
		/** Tapping photo `index`; without it the photos are pictures, not buttons. */
		onopen?: (index: number) => void;
		/** Photo `index` has loaded, at its natural size. With it every photo loads at once, so each is measured. */
		onmeasure?: (index: number, width: number, height: number) => void;
		/** Drawn over photo `index`: a remove button, an upload state. */
		children?: Snippet<[index: number]>;
	} = $props();

	const count = $derived(photos.length);
	// 1: one 4:3. 2: side by side. 3: one large left, two stacked right. 4: 2x2.
	const frame = $derived(
		[
			'aspect-[4/3]',
			'grid-cols-2 aspect-[3/2]',
			'grid-cols-[2fr_1fr] grid-rows-2 aspect-[3/2]',
			'grid-cols-2 grid-rows-2 aspect-[4/3]'
		][count - 1]
	);
</script>

<!-- How a comment's photos are posted, and how the composer previews them.
     Each cell is a box of its own, its picture laid in absolutely, so a cell
     never grows with its image and the frame keeps its ratio. The face is
     what the sortable lifts (`#lib/sortable`); it carries the corners. -->
<ul aria-label={m.diary_photos()} class="grid w-full max-w-full gap-1 md:w-96 {frame}">
	{#each photos as photo, index (photo.key)}
		<li
			data-sortable-key={photo.key}
			class="relative min-h-0 min-w-0 rounded-md {count === 3 && index === 0 ? 'row-span-2' : ''}"
		>
			<div data-sortable-face class="absolute inset-0 overflow-hidden rounded-md bg-skeleton">
				{#snippet picture()}
					<img
						src={photo.src}
						alt=""
						width={photo.width}
						height={photo.height}
						loading={onmeasure ? 'eager' : 'lazy'}
						decoding="async"
						draggable="false"
						onload={(event) => {
							const img = event.currentTarget as HTMLImageElement;
							onmeasure?.(index, img.naturalWidth, img.naturalHeight);
						}}
						class="size-full object-cover"
					/>
				{/snippet}
				{#if onopen}
					<button
						type="button"
						onclick={() => onopen(index)}
						aria-label={m.images_open({ number: index + 1, count })}
						class="block size-full rounded-md focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
					>
						{@render picture()}
					</button>
				{:else}
					{@render picture()}
				{/if}
				{@render children?.(index)}
			</div>
		</li>
	{/each}
</ul>
