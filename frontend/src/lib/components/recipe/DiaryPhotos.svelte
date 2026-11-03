<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import Check from '@lucide/svelte/icons/check';
	import CircleAlert from '@lucide/svelte/icons/circle-alert';
	import ImagePlus from '@lucide/svelte/icons/image-plus';
	import RotateCcw from '@lucide/svelte/icons/rotate-ccw';
	import X from '@lucide/svelte/icons/x';
	import { MAX_COMMENT_IMAGES } from '#lib/api/comments.js';
	import { ACCEPTED_IMAGE_TYPES } from '#lib/api/recipes.js';
	import Lightbox from '#lib/components/ui/Lightbox.svelte';
	import ProgressRing from '#lib/components/ui/ProgressRing.svelte';
	import { acceptFiles } from '#lib/images/pick.js';
	import { m } from '#lib/paraglide/messages.js';
	import { newId } from '#lib/recipe/form.js';
	import { revokePreviews, type PendingPhoto } from '#lib/recipe/photos.js';
	import { flip } from '#lib/sortable/flip.js';
	import { moveItem } from '#lib/sortable/order.js';
	import { sortable } from '#lib/sortable/sortable.svelte.js';
	import PhotoCollage from './PhotoCollage.svelte';

	let {
		photos = $bindable(),
		busy,
		title,
		hint = true
	}: {
		/** Picked files wait here until the caller stages them on Send. */
		photos: PendingPhoto[];
		/** A send is running: nothing may be added, removed or moved. */
		busy: boolean;
		/** The recipe's title, for the lightbox. */
		title: string;
		/** Draws the failed-photo line last in the caller's row; the composer draws it under its box instead. */
		hint?: boolean;
	} = $props();

	let collage = $state<HTMLElement>();
	let input = $state<HTMLInputElement>();
	let viewing = $state(false);
	let shown = $state(0);

	const max = MAX_COMMENT_IMAGES;
	const full = $derived(photos.length >= max);
	// Uploads run in order (stagePhotos), so the first unstaged photo is the one going up.
	const inFlight = $derived(
		busy ? photos.find((photo) => !photo.staged && !photo.failed)?.key : undefined
	);

	// The first photo that did not go up; tapping it sends again.
	const failedAt = $derived(photos.findIndex((photo) => photo.failed));

	onDestroy(() => revokePreviews(photos));

	async function add(files: File[]) {
		const accepted = acceptFiles(files, max - photos.length, m.diary_photos_limit({ count: max }));
		if (!accepted.length) return;
		// In at once, so the limit holds for the next paste and a Send sees
		// them; the size follows when the preview loads.
		photos = [
			...photos,
			...accepted.map((file) => ({
				key: newId(),
				file,
				preview: URL.createObjectURL(file),
				progress: 0,
				width: 0,
				height: 0
			}))
		];
		// The first photo moves the picker under the collage: a new input, which
		// takes the focus. The fourth disables it: the last remove button takes it.
		await tick();
		if (!full) input?.focus();
		else collage?.querySelector<HTMLButtonElement>('li:last-child button[data-remove]')?.focus();
	}

	/** Cmd/Ctrl+V of images into the caller's form adds them; anything else pastes as usual. */
	function pasteInto(node: HTMLElement) {
		const form = node.closest('form');
		const paste = (event: ClipboardEvent) => {
			const files = [...(event.clipboardData?.files ?? [])];
			if (!files.length || busy) return;
			event.preventDefault();
			void add(files);
		};
		form?.addEventListener('paste', paste);
		return () => form?.removeEventListener('paste', paste);
	}

	async function onchange() {
		if (!input?.files) return;
		const files = Array.from(input.files);
		input.value = '';
		await add(files);
	}

	async function remove(index: number) {
		revokePreviews([photos[index]]);
		photos = photos.filter((_, i) => i !== index);
		// The pressed button is gone: hand focus to its neighbor, or to the picker.
		await tick();
		const buttons = collage?.querySelectorAll<HTMLButtonElement>('button[data-remove]');
		(buttons?.[Math.min(index, buttons.length - 1)] ?? input)?.focus();
	}

	function open(index: number) {
		shown = index;
		viewing = true;
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
			disabled={busy || full}
			{onchange}
			class="sr-only"
		/>
	</label>
{/snippet}

{#snippet state(index: number)}
	{@const photo = photos[index]}
	{@const number = index + 1}
	{#if photo.failed && !busy}
		<!-- Submits the caller's form: the send uploads what is not up yet, then posts. -->
		<button
			type="submit"
			aria-label={m.comments_photo_failed({ number })}
			class="absolute inset-0 grid place-items-center rounded-md bg-destructive/35 ring-2 ring-destructive ring-inset focus-visible:outline-2 focus-visible:-outline-offset-4 focus-visible:outline-lightbox-foreground"
		>
			<span
				class="grid size-[30px] place-items-center rounded-full bg-destructive text-destructive-foreground"
			>
				<RotateCcw class="size-4" strokeWidth={2.5} aria-hidden="true" />
			</span>
		</button>
	{:else if photo.key === inFlight}
		<div class="absolute inset-0 grid place-items-center bg-overlay">
			<ProgressRing progress={photo.progress} size={30} />
		</div>
	{:else if busy && photo.file && photo.staged}
		<span
			class="absolute bottom-1 left-1 grid size-5 place-items-center rounded-full bg-success-soft text-success-foreground"
		>
			<Check class="size-[13px]" strokeWidth={3} aria-hidden="true" />
			<span class="sr-only">{m.comments_photo_uploaded()}</span>
		</span>
	{:else if busy && !photo.staged}
		<!-- Waiting its turn. -->
		<div class="absolute inset-0 bg-surface-elevated/55"></div>
	{/if}
	{#if !busy}
		<button
			type="button"
			data-remove
			data-sortable-ignore
			onclick={() => remove(index)}
			aria-label={m.diary_photo_remove({ n: number })}
			class="group absolute top-0 right-0 grid size-10 place-items-center transition-opacity focus-visible:outline-none in-data-resizing:opacity-0 in-data-resizing:transition-none"
		>
			<span
				class="grid size-6 place-items-center rounded-full border border-border bg-surface text-text shadow-card group-hover:text-destructive group-focus-visible:outline-2 group-focus-visible:outline-offset-1 group-focus-visible:outline-primary"
			>
				<X class="size-3.5" strokeWidth={2.5} aria-hidden="true" />
			</span>
		</button>
	{/if}
{/snippet}

<!-- `contents`: the collage and the picker are items of the caller's flex
     row. Without photos the picker is a round icon in that row; with photos
     the collage takes a row of its own, the picker the row under it. -->
<div class="contents" {@attach pasteInto}>
	{#if photos.length}
		<div class="w-full" data-collage>
			<!-- The sortable's contract (`#lib/sortable/sortable.svelte.ts`): no
			     selection, callout or pan stealing the long-press, and the held
			     photo's cell drawn as the dashed landing slot. -->
			<div
				bind:this={collage}
				use:sortable={{
					keys: photos.map((photo) => photo.key),
					disabled: () => busy,
					onmove: (from, to) => {
						void flip(collage, () => (photos = moveItem(photos, from, to)));
					},
					label: (_, position) => m.comments_photo_moved({ position, count: photos.length })
				}}
				class="[&_[data-sortable-key]]:touch-pan-y [&_[data-sortable-key]]:select-none [&_[data-sortable-key]]:[-webkit-tap-highlight-color:transparent] [&_[data-sortable-key]]:[-webkit-touch-callout:none] [&_[data-sortable-slot]]:outline-2 [&_[data-sortable-slot]]:-outline-offset-2 [&_[data-sortable-slot]]:outline-primary [&_[data-sortable-slot]]:outline-dashed"
			>
				<PhotoCollage
					photos={photos.map((photo) => ({
						key: photo.key,
						src: photo.preview,
						width: photo.width,
						height: photo.height
					}))}
					onopen={busy ? undefined : open}
					onmeasure={(index, width, height) => {
						if (!photos[index].width) Object.assign(photos[index], { width, height });
					}}
				>
					{#snippet children(index)}
						{@render state(index)}
					{/snippet}
				</PhotoCollage>
			</div>
			{#if !busy}
				<div class="mt-0.5 -ml-1 flex items-center gap-0.5">
					{@render picker('size-10 rounded-full')}
					<p class="text-micro text-text-muted tabular-nums">
						{#if photos.length > 1}
							<span class="pointer-coarse:hidden"
								>{m.comments_photos_hint_mouse({ count: photos.length, max })}</span
							>
							<span class="hidden pointer-coarse:inline"
								>{m.comments_photos_hint_touch({ count: photos.length, max })}</span
							>
						{:else}
							{photos.length}/{max}
						{/if}
					</p>
				</div>
			{/if}
		</div>
	{:else}
		{@render picker('size-10 rounded-full')}
	{/if}
	{#if hint && failedAt >= 0 && !busy}
		<p role="alert" class="order-last flex basis-full gap-1.5 text-body-sm text-destructive">
			<CircleAlert class="mt-0.5 size-4 shrink-0" aria-hidden="true" />
			{m.comments_photo_failed({ number: failedAt + 1 })}
		</p>
	{/if}
</div>

<!-- The picked photos large, before they are sent: local previews, nothing to download yet. -->
<Lightbox
	bind:open={viewing}
	bind:index={shown}
	recipeId=""
	{title}
	images={photos.map((photo) => ({ id: photo.key, width: photo.width, height: photo.height }))}
	src={(id) => photos.find((photo) => photo.key === id)?.preview ?? ''}
	download={false}
/>
