<script module lang="ts">
	const rest = { state: 'stored' as const, progress: 0, width: 0, height: 0 };
</script>

<script lang="ts">
	import { onDestroy } from 'svelte';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MediaQuery } from 'svelte/reactivity';
	import { scale } from 'svelte/transition';
	import { DropdownMenu } from 'bits-ui';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ArrowRight from '@lucide/svelte/icons/arrow-right';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import Eye from '@lucide/svelte/icons/eye';
	import LoaderCircle from '@lucide/svelte/icons/loader-circle';
	import Plus from '@lucide/svelte/icons/plus';
	import Star from '@lucide/svelte/icons/star';
	import Trash from '@lucide/svelte/icons/trash';
	import Upload from '@lucide/svelte/icons/upload';
	import { toast } from 'svelte-sonner';
	import {
		ACCEPTED_IMAGE_TYPES,
		MAX_IMAGES,
		deleteImage,
		imageUrl,
		reorderImages,
		uploadImage,
		type Image
	} from '#lib/api/recipes.js';
	import Lightbox from '#lib/components/ui/Lightbox.svelte';
	import { acceptFiles, uploadErrorMessage } from '#lib/images/pick.js';
	import { m } from '#lib/paraglide/messages.js';
	import { refusalOr } from '#lib/recipe/access.js';
	import { newId } from '#lib/recipe/form.js';
	import { flip } from '#lib/sortable/flip.js';
	import { moveItem } from '#lib/sortable/order.js';
	import { sortable } from '#lib/sortable/sortable.svelte.js';
	import { fresh } from './motion';

	let {
		recipeId,
		title,
		initialImages = [],
		pending = $bindable([]),
		oncount
	}: {
		/** Set for an existing recipe: every action calls the API at once. Unset: files are queued into `pending`. */
		recipeId?: string;
		/** The recipe's title, for the lightbox. */
		title: string;
		/** In their stored order: the first one is the cover. */
		initialImages?: Image[];
		/** New-recipe mode only: the queued files, in the order shown (the first becomes the cover). */
		pending?: File[];
		/** Told how many photos the section holds whenever that changes. */
		oncount?: (count: number) => void;
	} = $props();

	/**
	 * One tile; the first is the cover. Stored images carry their server id
	 * and thumbnail URL; queued or in-flight files carry the File plus an
	 * object URL that is revoked once the tile leaves the list or the
	 * section is destroyed. `key` stays when an upload lands and `id` turns
	 * into the server's, so the tile stays the same element. A queued tile
	 * of a new recipe is `stored` too: nothing is in flight for it.
	 */
	type Tile = {
		key: string;
		id: string;
		url: string;
		file?: File;
		state: 'stored' | 'uploading' | 'failed';
		progress: number;
		width: number;
		height: number;
	};

	// The props seed the section once (see the `svelte-ignore` comments);
	// everything after that is this component's own state, kept in step
	// with the server or with `pending`. In queued mode the tiles come back
	// from `pending`, so a remount keeps the queue.
	// svelte-ignore state_referenced_locally
	let tiles = $state<Tile[]>(
		recipeId === undefined
			? pending.map((file) => {
					const key = newId();
					return { key, id: key, url: URL.createObjectURL(file), file, ...rest };
				})
			: initialImages.map((image) => ({
					key: image.id,
					id: image.id,
					url: imageUrl(recipeId as string, image.id, 'thumb'),
					state: 'stored',
					progress: 1,
					width: image.width,
					height: image.height
				}))
	);
	// The order the server holds, by tile key: a failed save goes back to it.
	// svelte-ignore state_referenced_locally
	let savedKeys = tiles.map((tile) => tile.key);
	let dropping = $state(false);
	let viewing = $state(false);
	let viewIndex = $state(0);
	let announcement = $state('');
	let grid = $state<HTMLElement>();

	$effect.pre(() => {
		oncount?.(tiles.length);
	});

	const queued = $derived(recipeId === undefined);
	const full = $derived(tiles.length >= MAX_IMAGES);
	// Tiles still uploading sit at the end; nothing moves past the last stored one.
	const lastStored = $derived(tiles.findLastIndex((tile) => tile.state === 'stored'));
	const coarse = new MediaQuery('pointer: coarse');
	const hint = $derived(
		queued
			? m.images_queued_hint()
			: tiles.length < 2
				? m.images_hint()
				: coarse.current
					? m.images_hint_sort_touch()
					: m.images_hint_sort_mouse()
	);

	// Uploads run one after another: a second drop while the first batch is
	// still in flight simply extends the chain.
	let chain: Promise<void> = Promise.resolve();
	// One request to the recipe's photos at a time. The order request names
	// every stored photo, so an upload or delete landing while it is on the
	// way made the server refuse it; queued behind them it names what the
	// server holds by then. Uploads join one by one (through `chain`), so a
	// reorder waits for the upload in flight, not for the whole batch.
	let lock: Promise<unknown> = Promise.resolve();
	function serial<T>(run: () => Promise<T>): Promise<T> {
		const next = lock.then(run);
		lock = next.catch(() => {});
		return next;
	}

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
			const key = newId();
			fresh.add(key);
			tiles.push({
				key,
				id: key,
				url: URL.createObjectURL(file),
				file,
				...rest,
				state: queued ? 'stored' : 'uploading'
			});
			if (!queued) {
				chain = chain.then(() => upload(key));
			}
		}
		syncPending();
	}

	async function upload(key: string) {
		const id = recipeId as string;
		const tile = tiles.find((t) => t.key === key);
		if (!tile?.file) {
			return;
		}
		try {
			const image = await serial(() =>
				uploadImage(id, tile.file as File, (progress) => (tile.progress = progress))
			);
			revoke(tile);
			Object.assign(tile, {
				id: image.id,
				url: imageUrl(id, image.id, 'thumb'),
				file: undefined,
				state: 'stored',
				progress: 1,
				width: image.width,
				height: image.height
			});
		} catch (error) {
			toast.error(uploadErrorMessage(error));
			const index = tiles.indexOf(tile);
			if (index !== -1) {
				revoke(tile);
				tiles.splice(index, 1);
			}
		}
	}

	/** Saves the order on screen; `error` is the toast when the server refuses it. */
	async function saveOrder(error = m.images_reorder_error()) {
		syncPending();
		if (queued) {
			return;
		}
		const id = recipeId as string;
		try {
			const sent = await serial(async () => {
				const stored = tiles.filter((tile) => tile.state === 'stored');
				await reorderImages(
					id,
					stored.map((tile) => tile.id)
				);
				return stored.map((tile) => tile.key);
			});
			// The answer is the order sent, which the screen shows already.
			savedKeys = sent;
		} catch (err) {
			// Back to what the server holds; tiles it does not know yet
			// (uploading) stay at the end, in their order.
			const rank = new Map(savedKeys.map((key, i) => [key, i]));
			void flip(grid as HTMLElement, () => {
				tiles = tiles.toSorted(
					(a, b) => (rank.get(a.key) ?? MAX_IMAGES) - (rank.get(b.key) ?? MAX_IMAGES)
				);
			});
			toast.error(refusalOr(err, error));
		}
	}

	/** A menu move: the same slide and settle ring as a drag, then announced and saved. */
	function move(from: number, to: number, error = m.images_reorder_error()) {
		const key = tiles[from].key;
		const el = grid?.querySelector<HTMLElement>(`[data-sortable-key="${key}"]`) ?? null;
		void flip(grid as HTMLElement, () => (tiles = moveItem(tiles, from, to)), { settle: el });
		announcement = m.images_moved({ position: to + 1 });
		void saveOrder(error);
	}

	async function remove(index: number) {
		const tile = tiles[index];
		if (!queued) {
			try {
				await serial(() => deleteImage(recipeId as string, tile.id));
			} catch (error) {
				toast.error(refusalOr(error, m.images_delete_error()));
				return;
			}
		}
		await flip(grid as HTMLElement, () => {
			revoke(tile);
			tiles = tiles.filter((t) => t !== tile);
		});
		savedKeys = savedKeys.filter((key) => key !== tile.key);
		syncPending();
	}

	function view(index: number) {
		viewIndex = index;
		viewing = true;
	}

	function onchange(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		if (input.files) {
			addFiles(input.files);
		}
		input.value = '';
	}

	/** The whole section card takes dropped files; it carries `data-dropping` while they hover. */
	function dropTarget(node: HTMLElement) {
		const section = node.closest('section') ?? node;
		const set = (on: boolean) => {
			dropping = on;
			section.toggleAttribute('data-dropping', on);
		};
		const over = (event: DragEvent) => {
			if (!event.dataTransfer?.types.includes('Files')) return;
			event.preventDefault();
			set(true);
		};
		const leave = (event: DragEvent) => {
			if (!section.contains(event.relatedTarget as Node | null)) set(false);
		};
		const drop = (event: DragEvent) => {
			if (!dropping) return;
			event.preventDefault();
			set(false);
			if (event.dataTransfer?.files) addFiles(event.dataTransfer.files);
		};
		section.addEventListener('dragover', over);
		section.addEventListener('dragleave', leave);
		section.addEventListener('drop', drop);
		return () => {
			section.removeEventListener('dragover', over);
			section.removeEventListener('dragleave', leave);
			section.removeEventListener('drop', drop);
		};
	}

	function lightboxSrc(imageId: string, variant: 'thumb' | 'detail' | 'original') {
		const tile = tiles.find((t) => t.id === imageId);
		return tile?.file ? tile.url : imageUrl(recipeId as string, imageId, variant);
	}

	const menuItem =
		'flex min-h-10 items-center gap-2.5 rounded-sm px-2.5 text-body-sm font-medium outline-none data-disabled:opacity-40 data-highlighted:bg-accent data-highlighted:text-accent-foreground';
</script>

<div {@attach dropTarget}>
	<p class="mb-3.5 text-caption text-text-muted">{hint}</p>

	<div class="relative">
		<ul
			bind:this={grid}
			use:sortable={{
				keys: tiles.map((tile) => tile.key),
				disabled: (key) => tiles.find((tile) => tile.key === key)?.state !== 'stored',
				columns: () => getComputedStyle(grid as HTMLElement).gridTemplateColumns.split(' ').length,
				onmove: (from, to) => {
					void flip(
						grid as HTMLElement,
						() => (tiles = moveItem(tiles, from, Math.min(to, lastStored)))
					);
				},
				ondrop: () => void saveOrder(),
				label: (_key, position) => m.images_moved({ position })
			}}
			aria-label={m.editor_section_images()}
			class="grid grid-cols-3 gap-1.5 md:grid-cols-6 md:gap-2"
		>
			{#each tiles as tile, index (tile.key)}
				{@const radius = index === 0 ? 'rounded-lg' : 'rounded-md'}
				<!-- The intro is `|global`: the first photo creates the whole
				     list with it. `fresh` keeps a page load still. The item is
				     the landing slot while its face follows the pointer
				     (`#lib/sortable`), so rounding and clipping sit on the face. -->
				<li
					data-sortable-key={tile.key}
					in:scale|global={{
						start: 0.95,
						duration: fresh.delete(tile.key) && !prefersReducedMotion.current ? 220 : 0,
						easing: cubicOut
					}}
					aria-label={m.images_item_label({ number: index + 1 })}
					class="group relative aspect-square touch-pan-y select-none [-webkit-touch-callout:none] {radius} {index ===
					0
						? 'col-span-2 row-span-2'
						: ''} data-sortable-slot:bg-primary/8 data-sortable-slot:outline-2 data-sortable-slot:-outline-offset-2 data-sortable-slot:outline-primary data-sortable-slot:outline-dashed"
				>
					<div
						data-sortable-face
						class="absolute inset-0 overflow-hidden bg-skeleton transition-opacity {radius} {dropping
							? 'opacity-60'
							: ''}"
					>
						<button
							type="button"
							onclick={() => view(index)}
							aria-label={m.images_show({ number: index + 1 })}
							class="block size-full {tile.state === 'stored'
								? 'cursor-grab'
								: ''} focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary {radius}"
						>
							<img
								src={tile.url}
								alt=""
								decoding="async"
								onload={(event) => {
									// A local file's size, for the lightbox.
									const img = event.currentTarget as HTMLImageElement;
									if (tile.width === 0) {
										tile.width = img.naturalWidth;
										tile.height = img.naturalHeight;
									}
								}}
								class="size-full object-cover {tile.state === 'uploading' ? 'opacity-55' : ''}"
							/>
						</button>

						{#if tile.state === 'uploading'}
							<div
								class="pointer-events-none absolute inset-0 grid place-items-center bg-overlay text-inverse-foreground"
							>
								<LoaderCircle
									class="size-5 animate-spin motion-reduce:animate-none"
									aria-label={m.images_uploading()}
								/>
							</div>
						{:else}
							<!-- Hidden while `flip` resizes the tile, or they stretch with it. -->
							{#if index === 0}
								<span
									class="pointer-events-none absolute top-2 left-2 rounded-pill bg-primary px-2.5 py-0.5 text-label font-semibold tracking-wide text-primary-foreground group-data-resizing:opacity-0"
								>
									{m.images_cover_badge()}
								</span>
							{:else}
								<span
									aria-hidden="true"
									class="pointer-events-none absolute bottom-1.5 left-1.5 h-[18px] min-w-[18px] rounded-pill bg-surface/90 px-1.5 text-center text-label leading-[18px] font-semibold text-text-muted tabular-nums group-data-resizing:opacity-0"
								>
									{index + 1}
								</span>
							{/if}
							<DropdownMenu.Root>
								<DropdownMenu.Trigger
									aria-label={m.images_actions({ number: index + 1 })}
									class="group/more absolute top-0 right-0 grid size-10 place-items-center text-text outline-none group-data-resizing:opacity-0"
								>
									<span
										class="grid size-6 place-items-center rounded-pill bg-surface/90 shadow-card group-focus-visible/more:outline-2 group-focus-visible/more:outline-offset-2 group-focus-visible/more:outline-primary group-data-[state=open]/more:bg-surface group-data-[state=open]/more:outline-2 group-data-[state=open]/more:outline-offset-1 group-data-[state=open]/more:outline-primary"
									>
										<Ellipsis class="size-3.5" aria-hidden="true" />
									</span>
								</DropdownMenu.Trigger>
								<DropdownMenu.Portal>
									<DropdownMenu.Content
										preventScroll={false}
										sideOffset={2}
										align="end"
										class="z-50 min-w-46 rounded-md border border-popover-border bg-popover p-1.5 shadow-dialog"
									>
										<DropdownMenu.Item onSelect={() => view(index)} class="{menuItem} text-text">
											<Eye class="size-4" aria-hidden="true" />
											{m.images_view()}
										</DropdownMenu.Item>
										<DropdownMenu.Item
											disabled={index === 0}
											onSelect={() => move(index, 0, m.images_cover_error())}
											class="{menuItem} text-text"
										>
											<Star class="size-4" aria-hidden="true" />
											{m.images_set_cover()}
										</DropdownMenu.Item>
										<DropdownMenu.Item
											disabled={index === 0}
											onSelect={() => move(index, index - 1)}
											class="{menuItem} text-text"
										>
											<ArrowLeft class="size-4" aria-hidden="true" />
											{m.images_move_earlier()}
										</DropdownMenu.Item>
										<DropdownMenu.Item
											disabled={index >= lastStored}
											onSelect={() => move(index, index + 1)}
											class="{menuItem} text-text"
										>
											<ArrowRight class="size-4" aria-hidden="true" />
											{m.images_move_later()}
										</DropdownMenu.Item>
										<DropdownMenu.Separator class="mx-0.5 my-1 h-px bg-border" />
										<DropdownMenu.Item
											onSelect={() => remove(index)}
											class="{menuItem} text-destructive data-highlighted:bg-destructive-soft data-highlighted:text-destructive"
										>
											<Trash class="size-4" aria-hidden="true" />
											{m.common_remove()}
										</DropdownMenu.Item>
									</DropdownMenu.Content>
								</DropdownMenu.Portal>
							</DropdownMenu.Root>
						{/if}
					</div>
				</li>
			{/each}

			{#if !full}
				<li class={tiles.length === 0 ? 'col-span-2 row-span-2' : ''}>
					<label
						class="flex aspect-square size-full cursor-pointer flex-col items-center justify-center border-2 border-dashed bg-background p-1 text-center font-semibold transition focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-primary hover:border-primary {dropping
							? 'border-primary text-primary'
							: 'border-border text-text-muted'} {tiles.length === 0
							? 'gap-2 rounded-lg text-body'
							: 'gap-1.5 rounded-md text-caption'}"
					>
						<Plus
							class="text-handle {tiles.length === 0 ? 'size-7' : 'size-[22px]'}"
							aria-hidden="true"
						/>
						{#if tiles.length === 0}
							<span>{m.images_add_first()}</span>
							<span class="text-caption font-normal text-text-muted">{m.images_add_drop()}</span>
						{:else}
							<span>{m.images_add()}</span>
						{/if}
						<input
							type="file"
							multiple
							accept={ACCEPTED_IMAGE_TYPES.join(',')}
							aria-label={m.images_dropzone_label()}
							{onchange}
							class="sr-only"
						/>
					</label>
				</li>
			{/if}
		</ul>

		{#if dropping}
			<div class="pointer-events-none absolute inset-0 z-20 grid place-items-center">
				<span
					class="inline-flex items-center gap-2 rounded-pill bg-primary px-4 py-2.5 text-body-sm font-semibold text-primary-foreground shadow-cta"
				>
					<Upload class="size-4" aria-hidden="true" />
					{m.images_drop_release()}
				</span>
			</div>
		{/if}
	</div>

	<p class="sr-only" aria-live="polite">{announcement}</p>
</div>

<Lightbox
	bind:open={viewing}
	bind:index={viewIndex}
	recipeId={recipeId ?? ''}
	{title}
	images={tiles}
	src={lightboxSrc}
/>
