<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import CircleAlert from '@lucide/svelte/icons/circle-alert';
	import ImagePlus from '@lucide/svelte/icons/image-plus';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import X from '@lucide/svelte/icons/x';
	import { MAX_COMMENT_IMAGES } from '#lib/api/comments.js';
	import { ACCEPTED_IMAGE_TYPES } from '#lib/api/recipes.js';
	import { acceptFiles } from '#lib/images/pick.js';
	import { m } from '#lib/paraglide/messages.js';
	import { newId } from '#lib/recipe/form.js';
	import { revokePreviews, type PendingPhoto } from '#lib/recipe/photos.js';

	let {
		photos = $bindable(),
		busy
	}: {
		/** Picked files wait here until the caller stages them on Send. */
		photos: PendingPhoto[];
		/** A send is running: nothing may be added or removed. */
		busy: boolean;
	} = $props();

	let list = $state<HTMLUListElement>();
	let input = $state<HTMLInputElement>();

	const full = $derived(photos.length >= MAX_COMMENT_IMAGES);
	// Uploads run in order (stagePhotos), so the first unstaged photo is the one going up.
	const inFlight = $derived(busy ? photos.find((photo) => !photo.staged)?.key : undefined);

	onDestroy(() => revokePreviews(photos));

	async function onchange() {
		if (!input?.files) return;
		const accepted = acceptFiles(
			Array.from(input.files),
			MAX_COMMENT_IMAGES - photos.length,
			m.diary_photos_limit({ count: MAX_COMMENT_IMAGES })
		);
		input.value = '';
		photos = [
			...photos,
			...accepted.map((file) => ({ key: newId(), file, preview: URL.createObjectURL(file) }))
		];
		// The first photo turns the round picker into a tile: a new input, which
		// takes the focus. The last one takes the picker away: its tile's remove
		// button takes it then.
		await tick();
		(input ?? list?.querySelector<HTMLButtonElement>('li:last-child button'))?.focus();
	}

	async function remove(index: number) {
		revokePreviews([photos[index]]);
		photos = photos.filter((_, i) => i !== index);
		// The pressed button is gone: hand focus to its neighbor, or to the picker.
		await tick();
		const buttons = list?.querySelectorAll('button');
		(buttons?.[Math.min(index, buttons.length - 1)] ?? input)?.focus();
	}
</script>

{#snippet picker(shape: string)}
	<label
		class="inline-flex shrink-0 items-center justify-center text-text-muted transition has-focus-visible:outline-2 has-focus-visible:outline-primary has-enabled:cursor-pointer has-enabled:hover:text-primary has-disabled:opacity-50 {shape}"
	>
		<ImagePlus class="size-5" aria-hidden="true" />
		<span class="sr-only">{m.diary_attach()}</span>
		<input
			bind:this={input}
			type="file"
			multiple
			accept={ACCEPTED_IMAGE_TYPES.join(',')}
			disabled={busy}
			{onchange}
			class="sr-only"
		/>
	</label>
{/snippet}

<!-- `contents`: the tiles and the picker are items of the caller's flex row.
     Without photos the picker is a round icon in that row; with photos it is
     the last tile, so the field keeps the row's width for its prompt. -->
<div class="contents">
	{#if photos.length}
		<div class="w-full pt-1.5">
			<div class="flex flex-wrap gap-2">
				<ul bind:this={list} aria-label={m.diary_photos()} class="flex flex-wrap gap-2">
					{#each photos as photo, index (photo.key)}
						<li
							class="relative size-16 overflow-hidden rounded-md bg-skeleton {photo.failed
								? 'outline-2 outline-offset-1 outline-destructive'
								: ''}"
						>
							<img src={photo.preview} alt="" decoding="async" class="size-full object-cover" />
							{#if photo.key === inFlight}
								<div class="absolute inset-0 flex items-center justify-center bg-overlay">
									<LoaderCircle
										class="size-5 animate-spin text-lightbox-foreground motion-reduce:animate-none"
										aria-hidden="true"
									/>
									<span class="sr-only">{m.images_uploading()}</span>
								</div>
							{:else if photo.failed}
								<span
									class="absolute bottom-1 left-1 inline-flex size-[22px] items-center justify-center rounded-full bg-destructive text-destructive-foreground"
								>
									<CircleAlert class="size-4" aria-hidden="true" />
									<span class="sr-only">{m.images_upload_error()}</span>
								</span>
							{/if}
							<button
								type="button"
								disabled={busy}
								onclick={() => remove(index)}
								aria-label={m.diary_photo_remove({ n: index + 1 })}
								class="absolute top-1 right-1 inline-flex size-[26px] items-center justify-center rounded-full bg-surface text-text shadow-card transition hover:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-50"
							>
								<X class="size-4" aria-hidden="true" />
							</button>
						</li>
					{/each}
				</ul>
				{#if !full}
					{@render picker('size-16 rounded-md border-2 border-dashed border-border')}
				{/if}
			</div>
			{#if full}
				<p class="mt-1 text-micro text-text-muted">
					{m.diary_photos_limit({ count: MAX_COMMENT_IMAGES })}
				</p>
			{/if}
		</div>
	{:else}
		{@render picker('size-9 rounded-full')}
	{/if}
</div>
