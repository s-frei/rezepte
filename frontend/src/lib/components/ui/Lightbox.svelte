<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { fade } from 'svelte/transition';
	import ChevronLeft from 'lucide-svelte/icons/chevron-left';
	import ChevronRight from 'lucide-svelte/icons/chevron-right';
	import Download from 'lucide-svelte/icons/download';
	import X from 'lucide-svelte/icons/x';
	import { imageUrl, type Image } from '$lib/api/recipes';
	import { m } from '$lib/paraglide/messages';

	let {
		open = $bindable(false),
		index = $bindable(0),
		recipeId,
		title,
		images
	}: {
		open?: boolean;
		/** Index into `images` of the picture on screen. */
		index?: number;
		recipeId: string;
		title: string;
		images: Image[];
	} = $props();

	const SWIPE_THRESHOLD = 50;

	const count = $derived(images.length);
	const current = $derived(images[index]);

	function show(next: number) {
		if (count === 0) {
			return;
		}
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
		const delta = event.clientX - swipeStart;
		swipeStart = null;
		if (delta <= -SWIPE_THRESHOLD) {
			show(index + 1);
		} else if (delta >= SWIPE_THRESHOLD) {
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
		<Dialog.Content forceMount preventScroll={false} {onkeydown}>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen && current}
					<div
						{...props}
						class="fixed inset-0 z-50 flex flex-col bg-lightbox text-lightbox-foreground"
						transition:fade={{ duration: 200 }}
					>
						<header class="flex h-16 shrink-0 items-center gap-3 px-4 md:px-6">
							<Dialog.Title class="truncate font-display text-body font-medium"
								>{title}</Dialog.Title
							>
							<span class="ml-auto text-body-sm tabular-nums opacity-70">
								{m.lightbox_counter({ number: index + 1, count })}
							</span>
							<!-- `/images/...` is served by the Go binary, not by SvelteKit's
							     router, so there is no route id for `resolve()` to take. -->
							<!-- eslint-disable svelte/no-navigation-without-resolve -->
							<a
								href={imageUrl(recipeId, current.id, 'original')}
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
						</header>
						<Dialog.Description class="sr-only">{m.lightbox_title({ title })}</Dialog.Description>

						<!-- `role="group"`: a div carrying pointer handlers needs a role
						     (svelte a11y_no_static_element_interactions), and the swipe
						     area really is a group of image plus prev/next buttons. -->
						<div
							role="group"
							class="relative flex min-h-0 flex-1 items-center justify-center px-4 md:px-20"
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
							{#key current.id}
								<img
									src={imageUrl(recipeId, current.id, 'original')}
									alt={m.lightbox_thumbnail({ number: index + 1 })}
									width={current.width}
									height={current.height}
									decoding="async"
									class="max-h-full max-w-full rounded-2xl object-contain select-none"
									draggable="false"
								/>
							{/key}
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
							<div
								role="group"
								aria-label={m.images_thumbnails()}
								class="flex h-20 shrink-0 items-center justify-center gap-2 overflow-x-auto px-4"
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
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
