<script lang="ts">
	import { imageUrl, type Image } from '$lib/api/recipes';
	import { m } from '$lib/paraglide/messages';

	let {
		recipeId,
		title,
		images,
		coverId,
		layout,
		onopen
	}: {
		recipeId: string;
		title: string;
		/** At least one image - the page renders the placeholder otherwise. */
		images: Image[];
		/** The recipe's cover image: what the gallery shows first. */
		coverId: string | null;
		/** `mobile`: snap-scroll carousel with pager dots. `desktop`: 4:3 cover plus a thumbnail row. */
		layout: 'mobile' | 'desktop';
		/** Called with the index of the picture the user tapped/clicked. */
		onopen: (index: number) => void;
	} = $props();

	// Where the gallery opens. The cover is not always the first image, and a
	// recipe can have none at all (or one that is not in this list), which
	// `findIndex` reports as -1 - the first picture then. Reading the initial
	// `images` is the point: this is a starting position, not a binding.
	// svelte-ignore state_referenced_locally
	const coverIndex = Math.max(
		0,
		images.findIndex((image) => image.id === coverId)
	);

	// Index of the picture in view (carousel) or selected (desktop cover).
	let selected = $state(coverIndex);

	// Removing an image can leave `selected` past the end of a shorter list -
	// the carousel only learns its new position from a scroll event that a
	// removal does not always produce. Every read goes through this clamp, so
	// `images[active]` is in range for any non-empty list.
	const active = $derived(Math.max(0, Math.min(selected, images.length - 1)));

	// The scroller is the event's `currentTarget`, so tracking needs no
	// reference of its own: one snapped page is exactly one `clientWidth`.
	function trackScroll(event: UIEvent & { currentTarget: HTMLDivElement }) {
		const scroller = event.currentTarget;
		if (scroller.clientWidth === 0) {
			return;
		}
		selected = Math.round(scroller.scrollLeft / scroller.clientWidth);
	}

	// The carousel has no `selected` to render from - it scrolls - so the
	// cover is scrolled to when the scroller mounts. `coverIndex` is a plain
	// value, so the attachment has nothing to re-run on and every later
	// scroll stays the user's. The jump is instant on purpose: sliding past
	// every other picture would be the first thing the page does.
	function startAtCover(node: HTMLDivElement) {
		node.scrollTo({ left: coverIndex * node.clientWidth, behavior: 'auto' });
	}
</script>

{#if layout === 'mobile'}
	<div class="relative size-full">
		<div
			{@attach startAtCover}
			onscroll={trackScroll}
			class="flex size-full snap-x snap-mandatory [scrollbar-width:none] overflow-x-auto rounded-2xl"
		>
			{#each images as image, i (image.id)}
				<button
					type="button"
					onclick={() => onopen(i)}
					aria-label={m.images_open({ number: i + 1, count: images.length })}
					class="size-full shrink-0 snap-center"
				>
					<img
						src={imageUrl(recipeId, image.id, 'detail')}
						alt={i === coverIndex ? m.images_cover_alt({ title }) : ''}
						loading={i === coverIndex ? 'eager' : 'lazy'}
						decoding="async"
						class="size-full object-cover"
					/>
				</button>
			{/each}
		</div>
		{#if images.length > 1}
			<div class="absolute bottom-3 left-3 flex gap-1.5" aria-hidden="true">
				{#each images as image, i (image.id)}
					<span
						class="size-1.5 rounded-pill transition {i === active
							? 'bg-lightbox-foreground'
							: 'bg-lightbox-foreground/50'}"
					></span>
				{/each}
			</div>
		{/if}
	</div>
{:else}
	<div>
		<button
			type="button"
			onclick={() => onopen(active)}
			aria-label={m.images_open({ number: active + 1, count: images.length })}
			class="block aspect-[4/3] w-full overflow-hidden rounded-3xl shadow-cover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			{#key images[active].id}
				<img
					src={imageUrl(recipeId, images[active].id, 'detail')}
					alt={m.images_cover_alt({ title })}
					decoding="async"
					class="size-full object-cover"
				/>
			{/key}
		</button>
		{#if images.length > 1}
			<!-- `role="group"`: an `aria-label` on a bare `<div>` is ignored by
			     screen readers, a labelled group is announced. -->
			<div
				role="group"
				aria-label={m.images_thumbnails()}
				class="mt-3 flex gap-2 overflow-x-auto py-1"
			>
				{#each images as image, i (image.id)}
					<button
						type="button"
						onclick={() => (selected = i)}
						aria-label={m.images_show({ number: i + 1 })}
						aria-pressed={i === active}
						class="size-16 shrink-0 overflow-hidden rounded-md transition {i === active
							? 'outline-2 outline-offset-2 outline-primary'
							: 'opacity-80 hover:opacity-100'}"
					>
						<img
							src={imageUrl(recipeId, image.id, 'thumb')}
							alt=""
							loading="lazy"
							decoding="async"
							class="size-full object-cover"
						/>
					</button>
				{/each}
			</div>
		{/if}
	</div>
{/if}
