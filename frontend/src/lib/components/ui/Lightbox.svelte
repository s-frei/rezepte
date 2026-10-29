<script lang="ts">
	import { untrack, type Snippet } from 'svelte';
	import { Dialog } from 'bits-ui';
	import { fade, fly } from 'svelte/transition';
	import { prefersReducedMotion } from 'svelte/motion';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Download from '@lucide/svelte/icons/download';
	import X from '@lucide/svelte/icons/x';
	import { imageUrl, type Image, type ImageVariant } from '$lib/api/recipes';
	import { decoded } from '$lib/images/decoded';
	import { m } from '$lib/paraglide/messages';
	import { swipeDirection } from '$lib/recipe/swipe';
	import ScrollFade from './ScrollFade.svelte';

	let {
		open = $bindable(false),
		index = $bindable(0),
		recipeId,
		title,
		images,
		src = (imageId, variant) => imageUrl(recipeId, imageId, variant),
		action
	}: {
		open?: boolean;
		/** Index into `images` of the picture on screen. */
		index?: number;
		recipeId: string;
		title: string;
		images: Pick<Image, 'id' | 'width' | 'height'>[];
		/** One rendition's URL. Defaults to the signed-in `/images/...` route;
		 * the public share page passes its own `/public-images/...` route. */
		src?: (imageId: string, variant: ImageVariant) => string;
		/** Something to do with the picture on screen, in the header; it may take a row of its own. */
		action?: Snippet<[index: number]>;
	} = $props();

	const count = $derived(images.length);
	const current = $derived(images[index]);
	const fadeDuration = $derived(prefersReducedMotion.current ? 0 : 200);

	// The picture on screen follows `index` once the next one is decoded, and
	// the two cross over in one grid cell, sliding a little the way the reader
	// went: 1 forward, -1 back. The first picture of an opening shows at once.
	let shownId = $state<string | null>(null);
	let way = $state(1);
	const shown = $derived(images.find((image) => image.id === shownId) ?? current);
	const shift = $derived(prefersReducedMotion.current ? 0 : 24);
	$effect(() => {
		const id = current?.id;
		if (!open || !id) {
			shownId = null;
			return;
		}
		if (untrack(() => shownId) === null) {
			shownId = id;
			return;
		}
		let stale = false;
		decoded(src(id, 'original')).then(() => {
			if (!stale) shownId = id;
		});
		return () => (stale = true);
	});

	function show(next: number, direction = Math.sign(next - index) || 1) {
		if (count === 0) {
			return;
		}
		way = direction;
		index = (next + count) % count;
	}

	function onkeydown(event: KeyboardEvent) {
		// Escape is Bits UI's job (Dialog closes on it); arrows are ours.
		if (event.key === 'ArrowRight') {
			event.preventDefault();
			show(index + 1);
		} else if (event.key === 'ArrowLeft') {
			event.preventDefault();
			show(index - 1);
		}
	}

	// Basic pointer swipe: remember where the pointer went down, decide on up.
	let swipeStart: number | null = null;

	function onpointerdown(event: PointerEvent) {
		swipeStart = event.clientX;
	}

	function onpointerup(event: PointerEvent) {
		if (swipeStart === null) {
			return;
		}
		const swipe = swipeDirection(swipeStart, event.clientX);
		swipeStart = null;
		if (swipe === 'next') {
			show(index + 1);
		} else if (swipe === 'previous') {
			show(index - 1);
		}
	}

	// A gesture the browser takes over (scroll, pinch) or a pointer that leaves
	// the area never produces a `pointerup` here: forget the start point, or the
	// next unrelated `pointerup` would be measured against a stale one.
	function cancelSwipe() {
		swipeStart = null;
	}

	const iconButton =
		'inline-flex size-10 items-center justify-center rounded-full text-lightbox-foreground transition hover:bg-lightbox-foreground/15 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
</script>

<Dialog.Root bind:open>
	<Dialog.Portal>
		<!-- Scroll lock on, unlike the other dialogs: it covers the whole screen, so the page
		     shifting under it cannot be seen, while its scrollbar and the wheel would be. -->
		<Dialog.Content forceMount {onkeydown}>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen && current}
					<div
						{...props}
						class="fixed inset-0 z-50 flex flex-col bg-lightbox text-lightbox-foreground"
						transition:fade={{ duration: fadeDuration }}
					>
						<header class="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3 md:px-6">
							<!-- `flex-1` with a 0 basis: a long title truncates instead of wrapping the row. -->
							<Dialog.Title class="min-w-0 flex-1 truncate font-display text-body font-medium"
								>{title}</Dialog.Title
							>
							<span class="ml-auto text-body-sm tabular-nums opacity-70">
								{m.lightbox_counter({ number: index + 1, count })}
							</span>
							<!-- `/images/...` is served by the Go binary, not by SvelteKit's
							     router, so there is no route id for `resolve()` to take. -->
							<!-- eslint-disable svelte/no-navigation-without-resolve -->
							<a
								href={src(current.id, 'original')}
								download={`${title}-${index + 1}.jpg`}
								aria-label={m.lightbox_download()}
								class={iconButton}
							>
								<Download class="size-5" aria-hidden="true" />
							</a>
							<!-- eslint-enable svelte/no-navigation-without-resolve -->
							<Dialog.Close aria-label={m.lightbox_close()} class={iconButton}>
								<X class="size-5" aria-hidden="true" />
							</Dialog.Close>
							{@render action?.(index)}
						</header>
						<Dialog.Description class="sr-only">{m.lightbox_title({ title })}</Dialog.Description>

						<!-- Without the thumbnail strip below, the padding keeps the photo off the edge. -->
						<!-- `role="group"`: a div carrying pointer handlers needs a role
						     (svelte a11y_no_static_element_interactions), and the swipe
						     area really is a group of image plus prev/next buttons. -->
						<div
							role="group"
							class="relative flex min-h-0 flex-1 items-center justify-center px-4 md:px-20 {count >
							1
								? ''
								: 'pb-4 md:pb-6'}"
							{onpointerdown}
							{onpointerup}
							onpointercancel={cancelSwipe}
							onpointerleave={cancelSwipe}
						>
							{#if count > 1}
								<button
									type="button"
									onclick={() => show(index - 1)}
									aria-label={m.lightbox_previous()}
									class="absolute left-4 hidden size-12 items-center justify-center rounded-full bg-lightbox-foreground/15 text-lightbox-foreground transition hover:bg-lightbox-foreground/25 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:inline-flex"
								>
									<ChevronLeft class="size-6" aria-hidden="true" />
								</button>
							{/if}
							<div class="grid size-full grid-cols-[100%] grid-rows-[100%] place-items-center">
								{#key shown.id}
									<img
										src={src(shown.id, 'original')}
										alt={m.lightbox_thumbnail({ number: images.indexOf(shown) + 1 })}
										width={shown.width}
										height={shown.height}
										decoding="async"
										class="col-start-1 row-start-1 max-h-full max-w-full rounded-2xl object-contain select-none"
										draggable="false"
										in:fly={{ x: way * shift, duration: 160 }}
										out:fly={{ x: -way * shift, duration: 160 }}
									/>
								{/key}
							</div>
							{#if count > 1}
								<button
									type="button"
									onclick={() => show(index + 1)}
									aria-label={m.lightbox_next()}
									class="absolute right-4 hidden size-12 items-center justify-center rounded-full bg-lightbox-foreground/15 text-lightbox-foreground transition hover:bg-lightbox-foreground/25 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:inline-flex"
								>
									<ChevronRight class="size-6" aria-hidden="true" />
								</button>
							{/if}
						</div>

						{#if count > 1}
							<ScrollFade
								role="group"
								aria-label={m.images_thumbnails()}
								frame="shrink-0"
								fade="from-lightbox"
								class="flex h-20 items-center justify-center-safe gap-2 overflow-x-auto px-4"
							>
								{#each images as image, i (image.id)}
									<button
										type="button"
										onclick={() => show(i)}
										aria-label={m.lightbox_thumbnail({ number: i + 1 })}
										aria-current={i === index ? 'true' : undefined}
										class="size-12 shrink-0 overflow-hidden rounded-md transition md:size-14 {i ===
										index
											? 'outline-2 outline-offset-2 outline-primary'
											: 'opacity-50 hover:opacity-80'}"
									>
										<img
											src={src(image.id, 'thumb')}
											alt=""
											loading="lazy"
											decoding="async"
											class="size-full object-cover"
										/>
									</button>
								{/each}
							</ScrollFade>
						{/if}
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
