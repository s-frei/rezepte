<script lang="ts">
	import { onDestroy } from 'svelte';
	import { flip } from 'svelte/animate';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade, scale } from 'svelte/transition';
	import GripVertical from '@lucide/svelte/icons/grip-vertical';
	import ImagePlus from '@lucide/svelte/icons/image-plus';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import X from '@lucide/svelte/icons/x';
	import { dragHandle, dragHandleZone, type DndEvent } from 'svelte-dnd-action';
	import { toast } from 'svelte-sonner';
	import {
		ACCEPTED_IMAGE_TYPES,
		MAX_IMAGES,
		deleteImage,
		imageUrl,
		reorderImages,
		setCover,
		uploadImage,
		type Image
	} from '#lib/api/recipes.js';
	import { acceptFiles, uploadErrorMessage } from '#lib/images/pick.js';
	import { m } from '#lib/paraglide/messages.js';
	import { refusalOr } from '#lib/recipe/access.js';
	import { newId } from '#lib/recipe/form.js';
	import { fresh } from './motion';

	let {
		recipeId,
		initialImages = [],
		initialCoverId = null,
		pending = $bindable([]),
		oncount
	}: {
		/** Set for an existing recipe: every action calls the API at once. Unset: files are queued into `pending`. */
		recipeId?: string;
		initialImages?: Image[];
		initialCoverId?: string | null;
		/** New-recipe mode only: the queued files, in upload order. */
		pending?: File[];
		/** Told how many photos the section holds whenever that changes (the phone's contents sheet). */
		oncount?: (count: number) => void;
	} = $props();

	/**
	 * One tile. Stored images carry their server id and thumbnail URL; queued
	 * or in-flight files carry the File plus an object URL that is revoked
	 * once the tile leaves the list or the section is destroyed. `key` stays
	 * when an upload lands and `id` turns into the server's, so the tile
	 * stays the same element: its overlay fades off the photo instead of
	 * the whole tile being replaced (and its photo blanking while the
	 * thumbnail loads).
	 */
	type Tile = { key: string; id: string; url: string; file?: File; uploading: boolean };

	const FLIP_DURATION = $derived(prefersReducedMotion.current ? 0 : 150);
	const ARRIVE_DURATION = $derived(prefersReducedMotion.current ? 0 : 220);
	const OVERLAY_DURATION = $derived(prefersReducedMotion.current ? 0 : 200);

	// The props seed the section once: a `$state` initializer runs on the
	// first render only, so a later prop value never re-seeds and undoes the
	// user's edits - which is what the `svelte-ignore` comments say out loud.
	// Everything after that is this component's own state, kept in step with
	// the server (or with `pending`). In queued mode the tiles come back from
	// `pending`, the mirror `syncPending` writes - a remount keeps the queue
	// instead of wiping it on the next sync.
	// svelte-ignore state_referenced_locally
	let tiles = $state<Tile[]>(
		// `queued` is declared below, so the check is spelled out here.
		recipeId === undefined
			? pending.map((file) => ({
					key: newId(),
					id: newId(),
					url: URL.createObjectURL(file),
					file,
					uploading: false
				}))
			: initialImages.map((image) => ({
					key: image.id,
					id: image.id,
					url: imageUrl(recipeId as string, image.id, 'thumb'),
					uploading: false
				}))
	);
	// svelte-ignore state_referenced_locally
	let coverId = $state<string | null>(initialCoverId);
	let dragging = $state(false);

	$effect.pre(() => {
		oncount?.(tiles.length);
	});

	const queued = $derived(recipeId === undefined);
	const uploading = $derived(tiles.some((tile) => tile.uploading));
	const full = $derived(tiles.length >= MAX_IMAGES);
	// In queued mode the first tile is the future cover (see the props doc).
	const effectiveCoverId = $derived(queued ? (tiles[0]?.id ?? null) : coverId);

	// Uploads run one after another: a second drop while the first batch is
	// still in flight simply extends the chain.
	let chain: Promise<void> = Promise.resolve();

	function syncPending() {
		if (queued) {
			pending = tiles.flatMap((tile) => (tile.file ? [tile.file] : []));
		}
	}

	function revoke(tile: Tile) {
		if (tile.file) {
			URL.revokeObjectURL(tile.url);
		}
	}

	onDestroy(() => tiles.forEach(revoke));

	function addFiles(list: FileList | File[]) {
		for (const file of acceptFiles(
			Array.from(list),
			MAX_IMAGES - tiles.length,
			m.images_limit_reached({ count: MAX_IMAGES })
		)) {
			const id = newId();
			const tile: Tile = { key: id, id, url: URL.createObjectURL(file), file, uploading: !queued };
			fresh.add(tile.key);
			tiles.push(tile);
			if (!queued) {
				chain = chain.then(() => upload(tile));
			}
		}
		syncPending();
	}

	async function upload(tile: Tile) {
		if (!recipeId) {
			return;
		}
		const id = recipeId;
		try {
			const image = await uploadImage(id, tile.file as File);
			const index = tiles.findIndex((t) => t.id === tile.id);
			if (index === -1) {
				return; // removed while uploading
			}
			revoke(tiles[index]);
			tiles[index] = {
				key: tile.key,
				id: image.id,
				url: imageUrl(id, image.id, 'thumb'),
				uploading: false
			};
			if (coverId === null) {
				coverId = image.id; // the server made the first upload the cover
			}
		} catch (error) {
			toast.error(uploadErrorMessage(error));
			const index = tiles.findIndex((t) => t.id === tile.id);
			if (index !== -1) {
				revoke(tiles[index]);
				tiles.splice(index, 1);
			}
		}
	}

	async function remove(index: number) {
		const tile = tiles[index];
		if (tile.uploading) {
			return;
		}
		if (!queued && recipeId) {
			try {
				await deleteImage(recipeId, tile.id);
			} catch (error) {
				toast.error(refusalOr(error, m.images_delete_error()));
				return;
			}
		}
		// An upload finishing during the round trip above can have moved the
		// tile, so `index` is stale by now: look the tile up again and let the
		// refreshed list decide which image inherits the cover.
		const current = tiles.findIndex((t) => t.id === tile.id);
		if (current === -1) {
			return;
		}
		revoke(tiles[current]);
		tiles.splice(current, 1);
		if (coverId === tile.id) {
			coverId = tiles[0]?.id ?? null; // mirrors the server's promotion rule
		}
		syncPending();
	}

	async function makeCover(index: number) {
		const tile = tiles[index];
		if (tile.uploading) {
			return;
		}
		if (queued) {
			tiles.splice(index, 1);
			tiles.unshift(tile);
			syncPending();
			return;
		}
		if (!recipeId) {
			return;
		}
		try {
			await setCover(recipeId, tile.id);
			coverId = tile.id;
		} catch (error) {
			toast.error(refusalOr(error, m.images_cover_error()));
		}
	}

	// The order the running drag started from. Not `$state`: nothing renders
	// from it, it only survives from the first `consider` to `finalize`.
	let orderBeforeDrag: Tile[] | null = null;

	function consider(event: CustomEvent<DndEvent<Tile>>) {
		orderBeforeDrag ??= [...tiles];
		tiles = event.detail.items;
	}

	async function finalize(event: CustomEvent<DndEvent<Tile>>) {
		const before = orderBeforeDrag;
		orderBeforeDrag = null;
		tiles = event.detail.items;
		syncPending();
		if (queued || !recipeId) {
			return;
		}
		try {
			const ordered = await reorderImages(
				recipeId,
				tiles.map((tile) => tile.id)
			);
			// A file added while the request was in flight is not in the
			// server's answer: map the returned order onto the tiles that are
			// still here and keep every tile that still carries its file -
			// queued or in flight - at the end, where a new upload lands.
			const byId = new Map(tiles.map((tile) => [tile.id, tile]));
			const stored = ordered.flatMap((image) => {
				const tile = byId.get(image.id);
				return tile ? [tile] : [];
			});
			tiles = [...stored, ...tiles.filter((tile) => tile.file !== undefined)];
		} catch (error) {
			// The server kept the old order, so the screen goes back to it
			// rather than showing an order that only exists locally.
			if (before !== null) {
				tiles = before;
			}
			toast.error(refusalOr(error, m.images_reorder_error()));
		}
	}

	function onchange(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		if (input.files) {
			addFiles(input.files);
		}
		input.value = '';
	}

	function ondrop(event: DragEvent) {
		event.preventDefault();
		dragging = false;
		if (event.dataTransfer?.files) {
			addFiles(event.dataTransfer.files);
		}
	}

	function ondragover(event: DragEvent) {
		event.preventDefault();
		dragging = true;
	}

	const overlayButton =
		'rounded-pill bg-surface px-2.5 py-1 text-micro font-semibold text-text shadow-card transition hover-tint focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
</script>

<div class="space-y-4">
	<p class="text-caption text-text-muted">
		{queued ? m.images_queued_hint() : m.images_saved_immediately()}
	</p>

	{#if tiles.length > 0}
		<!-- The zone and item `aria-label`s are what svelte-dnd-action announces. -->
		<ul
			use:dragHandleZone={{
				items: tiles,
				type: 'photo',
				flipDurationMs: FLIP_DURATION,
				dropTargetStyle: {},
				dragDisabled: uploading
			}}
			onconsider={consider}
			onfinalize={finalize}
			aria-label={m.editor_section_images()}
			class="flex flex-wrap gap-3"
		>
			{#each tiles as tile, index (tile.key)}
				<!-- The intro is `|global`: the first photo creates the whole
				     list with it, and a local intro does not play for a block
				     created with its parent. `fresh` keeps a page load still. -->
				<li
					animate:flip={{ duration: FLIP_DURATION }}
					in:scale|global={{
						start: 0.95,
						duration: fresh.delete(tile.key) ? ARRIVE_DURATION : 0,
						easing: cubicOut
					}}
					aria-label={m.images_item_label({ number: index + 1 })}
					class="group relative h-[120px] w-[160px] list-none overflow-hidden rounded-lg bg-skeleton {tile.id ===
					effectiveCoverId
						? 'outline-2 outline-offset-2 outline-primary'
						: ''}"
				>
					<img src={tile.url} alt="" decoding="async" class="size-full object-cover" />

					{#if tile.uploading}
						<div
							out:fade={{ duration: OVERLAY_DURATION }}
							class="absolute inset-0 flex items-center justify-center gap-1.5 bg-overlay text-body-sm font-semibold text-inverse-foreground"
						>
							<LoaderCircle
								class="size-4 animate-spin motion-reduce:animate-none"
								aria-hidden="true"
							/>
							{m.images_uploading()}
						</div>
					{:else}
						{#if tile.id === effectiveCoverId}
							<span
								class="absolute top-2 left-2 rounded-pill bg-primary px-2 py-0.5 text-label font-semibold text-primary-foreground"
							>
								{m.images_cover_badge()}
							</span>
						{/if}
						<!-- Hidden until hover only where hovering exists: on a touch
						     screen the row stays visible, so nothing is tappable
						     without being seen. Keyboard focus reveals it either way. -->
						<div
							class="absolute inset-x-2 bottom-2 flex items-center gap-1.5 opacity-100 transition group-focus-within:opacity-100 group-hover:opacity-100 [@media(hover:hover)]:opacity-0"
						>
							<button
								use:dragHandle
								type="button"
								aria-label={m.images_reorder()}
								class="inline-flex size-[26px] cursor-grab items-center justify-center rounded-full bg-surface text-handle shadow-card focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
							>
								<GripVertical class="size-4" aria-hidden="true" />
							</button>
							{#if tile.id !== effectiveCoverId}
								<button type="button" onclick={() => makeCover(index)} class={overlayButton}>
									{m.images_set_cover()}
								</button>
							{/if}
							<button
								type="button"
								onclick={() => remove(index)}
								aria-label={m.images_remove()}
								class="ml-auto inline-flex size-[26px] items-center justify-center rounded-full bg-surface text-text shadow-card transition hover:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
							>
								<X class="size-4" aria-hidden="true" />
							</button>
						</div>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}

	{#if !full}
		<label
			{ondragover}
			ondragleave={() => (dragging = false)}
			{ondrop}
			class="flex min-h-[120px] cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed bg-background px-4 py-6 text-center text-body-sm font-medium text-text-muted transition {dragging
				? 'border-primary'
				: 'border-border'}"
		>
			<ImagePlus class="size-6 text-handle" aria-hidden="true" />
			<span class="mt-2">
				{m.images_dropzone_text()}
				<span class="font-semibold text-primary">{m.images_dropzone_action()}</span>
			</span>
			<input
				type="file"
				multiple
				accept={ACCEPTED_IMAGE_TYPES.join(',')}
				aria-label={m.images_dropzone_label()}
				{onchange}
				class="sr-only"
			/>
		</label>
	{/if}
</div>
