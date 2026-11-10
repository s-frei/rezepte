<script lang="ts">
	import { Dialog, Popover } from 'bits-ui';
	import type { Image, StepPhoto } from '#lib/api/recipes.js';
	import BottomSheet, { revealAbove, scrollBox } from '#lib/components/ui/BottomSheet.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { phone } from '#lib/ui/phone.svelte.js';
	import StepPhotoRow from './StepPhotoRow.svelte';

	// The photo a step's phrase opens: a card at the phrase on a desktop, a
	// bottom sheet on a phone (or wherever the caller asks for `sheet`).
	// "View large" closes it and hands the photo's position to the caller's
	// lightbox.
	let {
		open = $bindable(false),
		sheet,
		photo,
		anchor,
		images,
		src,
		stepNumber,
		onenlarge
	}: {
		open?: boolean;
		/** Always a sheet: cook mode, whose large screens show the photo beside the step instead. */
		sheet?: boolean;
		photo: StepPhoto | null;
		/** The phrase that was activated; the card sits at it. */
		anchor: HTMLElement | null;
		images: Image[];
		src: (imageId: string, variant: 'thumb' | 'detail') => string;
		stepNumber: number;
		onenlarge: (index: number) => void;
	} = $props();

	const index = $derived(photo ? images.findIndex((image) => image.id === photo.imageId) : -1);
	// Focus goes back to the phrase, which is no popover trigger Bits UI knows;
	// the cook page re-renders its step, so a phrase that is gone gets nothing.
	function restoreFocus(event: Event) {
		event.preventDefault();
		if (anchor?.isConnected) anchor.focus();
	}

	// A press on the phrase that opened the card is not "outside": closing on
	// the press and reopening on the click would flash it.
	function keepOnAnchor(event: Event) {
		if (event.target instanceof Node && anchor?.contains(event.target)) event.preventDefault();
	}

	let sheetBody = $state<HTMLElement | null>(null);

	/**
	 * The step the dim leaves out: its scroll box where it has one, cut to
	 * what shows of it, else its list item (number and text) or paragraph.
	 */
	const spotlight = $derived(
		anchor ? (scrollBox(anchor) ?? anchor.closest('li') ?? anchor.closest('p') ?? anchor) : null
	);

	/** A phone's sheet keeps the phrase in sight above it (`revealAbove`). */
	function reveal() {
		const box = sheetBody?.closest<HTMLElement>('[role="dialog"]');
		const phrase = anchor;
		if (!box || !phrase?.isConnected) return;
		revealAbove(box, phrase, () => {
			const rects = [...phrase.getClientRects()];
			return { top: rects[0].top, bottom: rects.at(-1)!.bottom };
		});
	}

	function enlarge(at: number) {
		open = false;
		onenlarge(at);
	}
</script>

{#if photo && (sheet || phone.value)}
	<BottomSheet
		bind:open
		closeLabel={m.step_photo_close()}
		{spotlight}
		onOpenAutoFocus={reveal}
		onCloseAutoFocus={restoreFocus}
		onInteractOutside={keepOnAnchor}
	>
		<!-- The photo keeps to about a third of the height, so the step still
		     shows above the sheet: on a phone held sideways, and in cook mode's
		     fallback on a large screen at a type size too large for the photo
		     beside the step. The column stays at least 22rem wide, so a short
		     window narrows the photo, not the caption and the row under it; an
		     upright phone keeps it at or near full width. `min-h-0 flex-1` as in
		     every sheet: without them the body cannot shrink to the sheet's
		     height and its bottom padding runs off the screen. -->
		<div
			bind:this={sheetBody}
			class="mx-auto min-h-0 w-full max-w-[max(22rem,calc(35dvh*4/3+2.5rem))] flex-1 overflow-y-auto px-5 pb-8"
		>
			<Dialog.Title class="mb-3 text-body-sm font-semibold text-text">
				{m.step_photo_caption({ step: stepNumber, phrase: photo.phrase })}
			</Dialog.Title>
			<img
				src={src(photo.imageId, 'detail')}
				alt={photo.phrase}
				class="aspect-[4/3] w-full max-w-[calc(35dvh*4/3)] rounded-lg object-cover"
			/>
			<div class="mt-2 flex items-center justify-between gap-3 text-caption">
				<StepPhotoRow {index} count={images.length} onenlarge={enlarge} />
			</div>
		</div>
	</BottomSheet>
{:else if photo}
	<Popover.Root bind:open>
		<Popover.Portal>
			<Popover.Content
				role="dialog"
				aria-label={m.step_photo_caption({ step: stepNumber, phrase: photo.phrase })}
				customAnchor={anchor}
				onCloseAutoFocus={restoreFocus}
				onInteractOutside={keepOnAnchor}
				side="bottom"
				sideOffset={8}
				collisionPadding={12}
				class="z-50 w-[min(22rem,calc(100vw-1.5rem))] overflow-hidden rounded-2xl border border-popover-border bg-popover text-text shadow-dialog"
			>
				<img
					src={src(photo.imageId, 'detail')}
					alt={photo.phrase}
					class="aspect-[4/3] w-full object-cover"
				/>
				<div class="flex items-center justify-between gap-3 px-4 py-3 text-caption">
					<StepPhotoRow {index} count={images.length} onenlarge={enlarge} />
				</div>
			</Popover.Content>
		</Popover.Portal>
	</Popover.Root>
{/if}
