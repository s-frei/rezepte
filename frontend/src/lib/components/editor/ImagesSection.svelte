<script module lang="ts">
	const rest = { state: 'stored' as const, progress: 0, width: 0, height: 0 };
</script>

<script lang="ts">
	import { onDestroy } from 'svelte';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MediaQuery, SvelteMap, SvelteSet } from 'svelte/reactivity';
	import { beforeNavigate } from '$app/navigation';
	import { scale } from 'svelte/transition';
	import { DropdownMenu } from 'bits-ui';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ArrowRight from '@lucide/svelte/icons/arrow-right';
	import CircleAlert from '@lucide/svelte/icons/circle-alert';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import Eye from '@lucide/svelte/icons/eye';
	import Plus from '@lucide/svelte/icons/plus';
	import RotateCcw from '@lucide/svelte/icons/rotate-ccw';
	import Star from '@lucide/svelte/icons/star';
	import Trash from '@lucide/svelte/icons/trash';
	import Upload from '@lucide/svelte/icons/upload';
	import { toast } from 'svelte-sonner';
	import {
		ACCEPTED_IMAGE_TYPES,
		MAX_IMAGES,
		deleteImage,
		getRecipe,
		imageUrl,
		reorderImages,
		uploadImage,
		type Image
	} from '#lib/api/recipes.js';
	import { ApiError } from '#lib/api/client.js';
	import Lightbox from '#lib/components/ui/Lightbox.svelte';
	import ProgressRing from '#lib/components/ui/ProgressRing.svelte';
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
	const storedTile = (image: Image, key = image.id): Tile => ({
		key,
		id: image.id,
		url: imageUrl(recipeId as string, image.id, 'thumb'),
		state: 'stored',
		progress: 1,
		width: image.width,
		height: image.height
	});
	// svelte-ignore state_referenced_locally
	let tiles = $state<Tile[]>(
		recipeId === undefined
			? pending.map((file) => {
					const key = newId();
					return { key, id: key, url: URL.createObjectURL(file), file, ...rest };
				})
			: initialImages.map((image) => storedTile(image))
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
	// Stored tiles are a prefix (uploading and failed ones follow); nothing moves past the last stored one.
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
	// One request to the recipe's photos at a time, each job committing its
	// own change to `tiles` before the next starts. The order request names
	// every photo the server holds, so an upload or delete landing while it
	// was on the way made the server refuse it; queued behind them it names
	// what the server holds by then. Uploads join one by one (through
	// `chain`), so a reorder waits for the upload in flight, not the batch.
	let lock: Promise<unknown> = Promise.resolve();
	function serial<T>(run: () => Promise<T>): Promise<T> {
		const next = lock.then(run);
		lock = next.catch(() => {});
		return next;
	}

	// Removed tiles waiting for their undo toast to close, by key. Off the
	// screen already; a stored one is still on the server until then.
	const pendingDeletes = new SvelteMap<string, Tile>();
	const onServer = (tile: Tile) => !queued && tile.state === 'stored';
	// What the server holds counts against the limit, a pending delete too.
	const used = $derived(
		tiles.length + [...pendingDeletes.values()].filter((tile) => onServer(tile)).length
	);
	const full = $derived(used >= MAX_IMAGES);
	const failed = $derived(tiles.findIndex((tile) => tile.state === 'failed'));

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

	// Leaving the editor deletes what is pending; a page unload sends the
	// requests at once, past the queue, so they outlive the page.
	beforeNavigate(({ willUnload }) => {
		if (willUnload) flushDeletes(true);
	});
	onDestroy(() => {
		flushDeletes();
		tiles.forEach(revoke);
	});

	function addFiles(list: FileList | File[]) {
		for (const file of acceptFiles(
			Array.from(list),
			MAX_IMAGES - used,
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
		if (!tile?.file || tile.state !== 'uploading') {
			return;
		}
		try {
			await serial(async () => {
				const image = await uploadImage(id, tile.file as File, (p) => (tile.progress = p));
				revoke(tile);
				Object.assign(tile, { ...storedTile(image, key), file: undefined });
				savedKeys = [...savedKeys, key];
				// The server appended it after the photos it holds: stored
				// tiles stay a prefix, ahead of a failed one still waiting.
				const index = tiles.indexOf(tile);
				const to = tiles.findIndex((t) => t.state !== 'stored');
				if (index !== -1 && to !== -1 && to < index) {
					await flip(grid, () => (tiles = moveItem(tiles, index, to)));
				}
			});
		} catch (error) {
			// It stays, marked, until it is retried or removed; the hint
			// under the grid says so. A reason a retry cannot fix (size,
			// type, a refusal) is worth a toast of its own.
			const reason = uploadErrorMessage(error);
			if (reason !== m.images_upload_error()) toast.error(reason);
			tile.state = 'failed';
			tile.progress = 0;
		}
	}

	function retry(key: string) {
		const tile = tiles.find((t) => t.key === key);
		if (tile?.state !== 'failed') return;
		tile.state = 'uploading';
		chain = chain.then(() => upload(key));
	}

	/** Saves the order on screen; `error` is the toast when the server refuses it. */
	async function saveOrder(error = m.images_reorder_error()) {
		syncPending();
		if (queued) {
			return;
		}
		const id = recipeId as string;
		try {
			await serial(async () => {
				// A pending delete is still on the server, which wants every
				// photo named: it goes last.
				const sent = [...tiles, ...pendingDeletes.values()].filter(onServer);
				await reorderImages(
					id,
					sent.map((tile) => tile.id)
				);
				// The answer is the order sent, which the screen shows already.
				savedKeys = sent.map((tile) => tile.key);
			});
		} catch (err) {
			toast.error(refusalOr(err, error));
			// A 422: the server holds other photos than the ones sent (another
			// member added or removed some), so `savedKeys` is stale too.
			if (err instanceof ApiError && err.status === 422) {
				try {
					await serial(reload);
					return;
				} catch (reloadError) {
					// What the server holds is unknown: back to the last order it
					// accepted, below. The toast above already says it failed.
					console.error(reloadError);
				}
			}
			// Back to what the server holds; tiles it does not know yet
			// (uploading) stay at the end, in their order.
			const rank = new Map(savedKeys.map((key, i) => [key, i]));
			void flip(grid, () => {
				tiles = tiles.toSorted(
					(a, b) => (rank.get(a.key) ?? MAX_IMAGES) - (rank.get(b.key) ?? MAX_IMAGES)
				);
			});
		}
	}

	/**
	 * Shows the photos the server holds, in its order, keeping the tiles it
	 * still knows; uploading and failed ones stay at the end. A pending
	 * delete stays off the screen, or is dropped once the photo is gone.
	 */
	async function reload() {
		const { images } = await getRecipe(recipeId as string);
		const known = new Map(
			[...tiles, ...pendingDeletes.values()].filter(onServer).map((t) => [t.id, t])
		);
		const held = new Set(images.map((image) => image.id));
		for (const [key, tile] of pendingDeletes) {
			if (!held.has(tile.id) && !deleting.has(key)) pendingDeletes.delete(key);
		}
		const all = images.map((image) => known.get(image.id) ?? storedTile(image));
		savedKeys = all.map((tile) => tile.key);
		await flip(grid, () => {
			tiles = [
				...all.filter((tile) => !pendingDeletes.has(tile.key)),
				...tiles.filter((tile) => !onServer(tile))
			];
		});
	}

	/** A menu move: the same slide and settle ring as a drag, then announced and saved. */
	function move(from: number, to: number, error = m.images_reorder_error()) {
		const key = tiles[from].key;
		const el = grid?.querySelector<HTMLElement>(`[data-sortable-key="${key}"]`) ?? null;
		void flip(grid, () => (tiles = moveItem(tiles, from, to)), { settle: el });
		announcement = m.images_moved({ position: to + 1 });
		void saveOrder(error);
	}

	/**
	 * Takes the tile off the screen at once. A failed upload is simply gone;
	 * anything else waits in `pendingDeletes` behind an undo toast, and is
	 * deleted when the toast closes or the editor is left.
	 */
	function remove(index: number) {
		const tile = tiles[index];
		void flip(grid, () => (tiles = tiles.filter((t) => t !== tile)));
		syncPending();
		if (tile.state === 'failed') {
			revoke(tile);
			return;
		}
		pendingDeletes.set(tile.key, tile);
		const toastId = toast(m.images_removed(), {
			action: { label: m.images_undo(), onClick: () => undo(tile, index) },
			onAutoClose: () => flushDelete(tile.key),
			onDismiss: () => flushDelete(tile.key)
		});
		toastIds.set(tile.key, toastId);
	}

	// Each pending removal's toast, to close it on a flush.
	const toastIds = new SvelteMap<string, string | number>();

	function undo(tile: Tile, index: number) {
		if (deleting.has(tile.key) || !pendingDeletes.delete(tile.key)) return;
		toastIds.delete(tile.key);
		// Back where it was, on its side of the stored prefix: a stored tile
		// never behind one the server does not hold, any other never inside it.
		const unsettled = tiles.findIndex((t) => t.state !== 'stored');
		const prefix = unsettled === -1 ? tiles.length : unsettled;
		const at = tile.state === 'stored' ? Math.min(index, prefix) : Math.max(index, prefix);
		void flip(grid, () => {
			tiles = tiles.toSpliced(at, 0, tile);
		});
		syncPending();
		// Moves made meanwhile sent it last; put the server back in step.
		const now = [...tiles, ...pendingDeletes.values()].filter(onServer).map((t) => t.key);
		if (now.join() !== savedKeys.join()) void saveOrder();
	}

	// Pending deletes whose request is queued or on its way. They stay in
	// `pendingDeletes` until the server answers, so an order request queued
	// ahead of the delete still names them (the server holds them till then).
	const deleting = new SvelteSet<string>();

	function flushDelete(key: string, unloading = false) {
		const tile = pendingDeletes.get(key);
		if (!tile || (deleting.has(key) && !unloading)) return;
		const toastId = toastIds.get(key);
		toastIds.delete(key);
		if (toastId !== undefined) toast.dismiss(toastId);
		if (!onServer(tile)) {
			pendingDeletes.delete(key);
			revoke(tile);
			return;
		}
		const id = recipeId as string;
		if (unloading) {
			pendingDeletes.delete(key);
			void deleteImage(id, tile.id, { keepalive: true }).catch(() => {});
			return;
		}
		deleting.add(key);
		serial(async () => {
			await deleteImage(id, tile.id);
			pendingDeletes.delete(key);
			savedKeys = savedKeys.filter((k) => k !== key);
		})
			.catch((error) => {
				// Still on the server: show it again where the server keeps it,
				// behind the stored tiles it ranks after there.
				pendingDeletes.delete(key);
				const rank = savedKeys.indexOf(key);
				const at = tiles.filter((t) => onServer(t) && savedKeys.indexOf(t.key) < rank).length;
				void flip(grid, () => (tiles = tiles.toSpliced(at, 0, tile)));
				toast.error(refusalOr(error, m.images_delete_error()));
			})
			.finally(() => deleting.delete(key));
	}

	function flushDeletes(unloading = false) {
		for (const key of [...pendingDeletes.keys()]) flushDelete(key, unloading);
	}

	/**
	 * Sends every pending delete and resolves once the queue has settled, so
	 * the page loaded next no longer shows them. Await it before leaving.
	 */
	export async function flush(): Promise<void> {
		flushDeletes();
		// Uploads join the queue one by one: the chain holds those not in it yet.
		await chain;
		await lock;
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

	/**
	 * Pasted image files land here wherever the editor has the focus, except
	 * in a text field: Safari focuses no button on click, so the focus is
	 * often on the page itself, not in this section.
	 */
	function onpaste(event: ClipboardEvent) {
		const target = event.target;
		if (
			target instanceof HTMLInputElement ||
			target instanceof HTMLTextAreaElement ||
			(target instanceof HTMLElement && target.isContentEditable)
		) {
			return;
		}
		const files = [...(event.clipboardData?.files ?? [])].filter((file) =>
			file.type.startsWith('image/')
		);
		if (files.length === 0) return;
		event.preventDefault();
		addFiles(files);
	}

	function lightboxSrc(imageId: string, variant: 'thumb' | 'detail' | 'original') {
		const tile = tiles.find((t) => t.id === imageId);
		return tile?.file ? tile.url : imageUrl(recipeId as string, imageId, variant);
	}

	const menuItem =
		'flex min-h-10 items-center gap-2.5 rounded-sm px-2.5 text-body-sm font-medium outline-none data-disabled:opacity-40 data-highlighted:bg-accent data-highlighted:text-accent-foreground';
</script>

<svelte:window {onpaste} />

<div {@attach dropTarget}>
	<p class="mb-3.5 text-caption text-text-muted">{hint}</p>

	<div class="relative">
		<ul
			bind:this={grid}
			use:sortable={{
				keys: tiles.map((tile) => tile.key),
				disabled: (key) => tiles.find((tile) => tile.key === key)?.state !== 'stored',
				onmove: (from, to) => (tiles = moveItem(tiles, from, Math.min(to, lastStored))),
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
					class="group relative aspect-square touch-pan-y select-none [-webkit-tap-highlight-color:transparent] [-webkit-touch-callout:none] {radius} {index ===
					0
						? 'col-span-2 row-span-2'
						: ''} data-sortable-slot:bg-primary/8 data-sortable-slot:outline-2 data-sortable-slot:-outline-offset-2 data-sortable-slot:outline-primary data-sortable-slot:outline-dashed"
				>
					<div
						data-sortable-face
						class="absolute inset-0 overflow-hidden bg-skeleton transition-opacity {radius} {dropping
							? 'opacity-60'
							: ''} {tile.state === 'failed'
							? 'outline-2 outline-offset-1 outline-destructive'
							: ''}"
					>
						<button
							type="button"
							onclick={() => (tile.state === 'failed' ? retry(tile.key) : view(index))}
							aria-label={tile.state === 'failed'
								? m.images_retry_photo({ number: index + 1 })
								: m.images_show({ number: index + 1 })}
							class="block size-full {tile.state === 'stored'
								? 'cursor-grab group-data-lifted:cursor-grabbing'
								: ''} focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary {radius}"
						>
							<img
								src={tile.url}
								alt=""
								decoding="async"
								draggable="false"
								onload={(event) => {
									// A local file's size, for the lightbox.
									const img = event.currentTarget as HTMLImageElement;
									if (tile.width === 0) {
										tile.width = img.naturalWidth;
										tile.height = img.naturalHeight;
									}
								}}
								class="size-full object-cover [-webkit-user-drag:none] {tile.state === 'uploading'
									? 'opacity-55'
									: ''}"
							/>
						</button>

						{#if tile.state === 'uploading'}
							<div class="pointer-events-none absolute inset-0 grid place-items-center bg-overlay">
								<ProgressRing progress={tile.progress} size={40} />
							</div>
						{:else}
							<!-- Hidden while `flip` resizes the tile, or they stretch with it. -->
							{#if tile.state === 'failed'}
								<span class="pointer-events-none absolute inset-0 bg-destructive/35"></span>
								<span
									class="pointer-events-none absolute bottom-1.5 left-1.5 inline-flex h-[22px] items-center gap-1 rounded-pill bg-destructive pr-2 pl-1.5 text-micro font-semibold text-destructive-foreground max-[340px]:w-[22px] max-[340px]:justify-center max-[340px]:px-0"
								>
									<RotateCcw class="size-[13px] stroke-[2.5]" aria-hidden="true" />
									<span class="max-[340px]:hidden">{m.images_retry()}</span>
								</span>
							{:else if index === 0}
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
									data-sortable-ignore
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
										{#if tile.state === 'failed'}
											<DropdownMenu.Item
												onSelect={() => retry(tile.key)}
												class="{menuItem} text-text"
											>
												<RotateCcw class="size-4" aria-hidden="true" />
												{m.images_retry()}
											</DropdownMenu.Item>
										{:else}
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
										{/if}
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

	{#if failed !== -1}
		<p role="alert" class="mt-3 flex items-start gap-1.5 text-caption text-destructive">
			<CircleAlert class="mt-px size-4 shrink-0" aria-hidden="true" />
			{(coarse.current ? m.images_failed_hint : m.images_failed_hint_mouse)({ number: failed + 1 })}
		</p>
	{/if}

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
