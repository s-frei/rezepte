<script lang="ts">
	import { untrack } from 'svelte';
	import { Dialog, Popover } from 'bits-ui';
	import ImageIcon from '@lucide/svelte/icons/image';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { PhotoTile } from '#lib/recipe/step-photos.js';
	import { swipeDirection } from '#lib/recipe/swipe.js';

	/**
	 * Picks the photo a step's phrase links to: the chosen photo large and
	 * uncropped, a strip of every photo to choose from, and one button that
	 * links it. On a phone the app's bottom sheet, elsewhere the editor's popup
	 * frame hung below `anchor`. Opens on the photo already linked, else on the
	 * first.
	 */
	let {
		open = $bindable(false),
		phrase,
		tiles,
		selected,
		phone,
		anchor,
		spotlight,
		onreveal,
		onpick
	}: {
		open: boolean;
		/** The words the photo is for. */
		phrase: string;
		/** The recipe's photos, in their order. */
		tiles: PhotoTile[];
		/** The key of the photo linked to the phrase now, if any. */
		selected: string | null;
		phone: boolean;
		/** Where the desktop popup hangs, in viewport coordinates. */
		anchor: { left: number; top: number } | null;
		onpick: (key: string) => void;
		/** On a phone: what the dim leaves out, the step being linked (`BottomSheet`'s `spotlight`). */
		spotlight: HTMLElement | null;
		/** On a phone, once the sheet is up: scroll the phrase clear of it (the sheet's box). */
		onreveal: (sheet: HTMLElement) => void;
	} = $props();

	// Each opening starts on the linked photo, or on the first; a tap in the
	// strip or a swipe moves it. Held by key, so a photo added, restored or
	// moved meanwhile leaves the choice on the same photo; the chosen one
	// gone, it falls back to the first.
	let chosen = $derived(open ? untrack(() => selected) : null);
	const index = $derived(
		Math.max(
			0,
			tiles.findIndex((tile) => tile.key === chosen)
		)
	);
	let strip = $state<HTMLElement | null>(null);

	const current = $derived(tiles[index]);

	// The chosen thumbnail scrolls into view, wherever in the strip it is.
	$effect(() => {
		strip
			?.querySelector(`[data-at="${index}"]`)
			?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
	});

	const show = (at: number) =>
		(chosen = tiles[Math.max(0, Math.min(at, tiles.length - 1))]?.key ?? null);

	// A swipe on the large photo pages, as in the lightbox.
	let swipeStart: number | null = null;
	function onpointerup(event: PointerEvent) {
		if (swipeStart === null) return;
		const swipe = swipeDirection(swipeStart, event.clientX);
		swipeStart = null;
		if (swipe === 'next') show(index + 1);
		else if (swipe === 'previous') show(index - 1);
	}

	function confirm() {
		onpick(current.key);
		open = false;
	}

	/** Focus starts on the chosen thumbnail, which the arrows and Tab go on from. */
	function focusChosen(event: Event) {
		event.preventDefault();
		strip?.querySelector<HTMLElement>(`[data-at="${index}"]`)?.focus();
	}

	/** The arrows walk the strip. */
	function stripKeys(event: KeyboardEvent) {
		const delta = { ArrowRight: 1, ArrowLeft: -1 }[event.key];
		if (delta === undefined) return;
		event.preventDefault();
		show(index + delta);
		strip?.querySelector<HTMLElement>(`[data-at="${index}"]`)?.focus();
	}

	/** The point the desktop popup hangs from, as an element Bits UI can place against. */
	const virtualAnchor = $derived(
		anchor && {
			getBoundingClientRect: () => new DOMRect(anchor.left, anchor.top, 0, 0)
		}
	);
</script>

<!--
	On a phone the body is a column that fits the sheet: header and strip keep
	their height, the large photo's 4:3 box shrinks (`min-h-0`) and the photo
	shrinks inside it, centered and rounded itself, so no band shows beside it;
	the button stays in a footer below it. A long phrase's header is clamped;
	the sheet's title carries it whole.
-->
{#snippet body()}
	<div class="flex shrink-0 items-baseline justify-between gap-3">
		<p class="line-clamp-2 min-w-0 text-body-sm text-text-muted">
			{m.editor_photo_for({ phrase })}
		</p>
		<p aria-hidden="true" class="shrink-0 text-caption text-text-muted tabular-nums">
			{index + 1} / {tiles.length}
		</p>
	</div>
	{#if current}
		<!--
			`role="group"`: a div carrying pointer handlers needs a role. `select-none`
			and `touch-pan-y`, as on the lightbox: otherwise a mouse swipe selects and
			a phone takes the finger's horizontal pan before the swipe is measured.
		-->
		<div
			role="group"
			class="mt-2 flex aspect-4/3 min-h-0 touch-pan-y items-center justify-center select-none"
			onpointerdown={(event) => (swipeStart = event.clientX)}
			{onpointerup}
			onpointercancel={() => (swipeStart = null)}
			onpointerleave={() => (swipeStart = null)}
		>
			<img
				src={current.detail}
				alt=""
				draggable="false"
				class="pointer-events-none max-h-full max-w-full rounded-md"
			/>
		</div>
	{/if}
	<!-- `p-1`: room for the chosen one's outline, which the scroll box would clip. -->
	<div
		bind:this={strip}
		role="group"
		aria-label={m.editor_photo_for({ phrase })}
		class="-mx-1 mt-2 flex shrink-0 gap-2 overflow-x-auto overscroll-contain p-1"
	>
		{#each tiles as tile, at (tile.key)}
			<button
				type="button"
				data-at={at}
				aria-pressed={at === index}
				aria-label="{at + 1} / {tiles.length}"
				tabindex={at === index ? 0 : -1}
				onclick={() => (chosen = tile.key)}
				onkeydown={stripKeys}
				class="shrink-0 overflow-hidden rounded-sm outline-offset-2 transition focus-visible:outline-2 focus-visible:outline-primary {phone
					? 'size-16'
					: 'size-12'} {at === index
					? 'outline-2 outline-primary'
					: 'opacity-80 hover:opacity-100'}"
			>
				<img src={tile.thumb} alt="" draggable="false" class="size-full object-cover" />
			</button>
		{/each}
	</div>
{/snippet}

{#snippet action()}
	<Button size={phone ? 'lg' : 'md'} class="w-full gap-2" onclick={confirm}>
		<ImageIcon class="size-4.5 shrink-0" aria-hidden="true" />
		{m.editor_photo_confirm({ number: index + 1 })}
	</Button>
{/snippet}

{#if phone}
	<BottomSheet
		bind:open
		closeLabel={m.common_close()}
		{spotlight}
		onOpenAutoFocus={(event) => {
			focusChosen(event);
			const box = strip?.closest<HTMLElement>('[role="dialog"]');
			if (box) onreveal(box);
		}}
		onCloseAutoFocus={(event) => event.preventDefault()}
	>
		<div class="flex min-h-0 flex-1 flex-col overflow-y-auto px-5 pt-1">
			<Dialog.Title class="sr-only">{m.editor_photo_for({ phrase })}</Dialog.Title>
			{@render body()}
		</div>
		<div class="shrink-0 px-5 pt-3 pb-8">{@render action()}</div>
	</BottomSheet>
{:else}
	<!--
		In place rather than in a portal, like "Mark as time": focus inside it is
		still focus in the step, so the step's actions stay while it is open.
	-->
	<Popover.Root bind:open>
		<Popover.Content
			customAnchor={virtualAnchor}
			side="bottom"
			align="start"
			sideOffset={4}
			collisionPadding={8}
			strategy="fixed"
			trapFocus={false}
			aria-label={m.editor_photo_for({ phrase })}
			onOpenAutoFocus={focusChosen}
			onCloseAutoFocus={(event) => event.preventDefault()}
			class="z-50 w-80 max-w-[calc(100vw-1rem)] rounded-md border border-primary bg-surface-elevated p-3 shadow-card"
		>
			{@render body()}
			<div class="mt-3">{@render action()}</div>
		</Popover.Content>
	</Popover.Root>
{/if}
