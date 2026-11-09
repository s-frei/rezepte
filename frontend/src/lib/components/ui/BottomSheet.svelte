<script lang="ts">
	import { Dialog } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade, fly } from 'svelte/transition';
	import SheetHandle from './SheetHandle.svelte';

	// Shared Overlay + Content wrapper for the phone's bottom sheets (the
	// contents, You, one person), so the phone has one kind of sheet: a
	// rounded panel rising from the bottom edge, its handle closing it on a
	// tap or a pull.
	// The caller renders the title and the scrolling body inside it.
	let {
		open = $bindable(false),
		closeLabel,
		onOpenAutoFocus,
		onCloseAutoFocus,
		tall = false,
		spotlight = null,
		children
	}: {
		open?: boolean;
		/** Accessible name of the handle, which closes the sheet. */
		closeLabel: string;
		/** Forwarded to Bits UI: where focus goes once the sheet has opened. */
		onOpenAutoFocus?: (event: Event) => void;
		/** Forwarded to Bits UI: where focus goes once the sheet has closed. */
		onCloseAutoFocus?: (event: Event) => void;
		/**
		 * Up to 92dvh instead of 80dvh, for a sheet whose content is the
		 * point of opening it (the pass-on sheet): Safari's bars leave an
		 * iPhone about 660px, and 80dvh of that dropped the pass-on ticket.
		 */
		tall?: boolean;
		/**
		 * An element the dim leaves out: it stays as bright as the sheet and
		 * takes taps, for a sheet about something on the page, such as a
		 * step's word. A click there is outside the sheet for Bits UI and
		 * closes it; a caller that wants a tap there to do more answers it
		 * first (the step editor at `touchend`).
		 */
		spotlight?: HTMLElement | null;
		children: Snippet;
	} = $props();

	const duration = $derived(prefersReducedMotion.current ? 0 : 250);
	const fadeDuration = $derived(prefersReducedMotion.current ? 0 : 200);

	/**
	 * The dim with a hole where `spotlight` is: a clip path, so the hole is
	 * clipped out of the hit test too and a tap there reaches the element.
	 * Followed on scroll and resize, since the page under the sheet scrolls.
	 */
	let hole = $state<string | undefined>(undefined);
	function measure() {
		if (!spotlight) {
			hole = undefined;
			return;
		}
		const r = spotlight.getBoundingClientRect();
		const k = Math.min(
			parseFloat(getComputedStyle(spotlight).borderTopLeftRadius) || 0,
			r.height / 2
		);
		const arc = (x: number, y: number) => `A${k} ${k} 0 0 1 ${x} ${y}`;
		hole =
			// The outer edge reaches far past any screen: a zoomed page's
			// innerWidth is smaller than the dim, which would then cover only
			// its top left.
			`path(evenodd, "M-1e5 -1e5H1e5V1e5H-1e5Z ` +
			`M${r.left + k} ${r.top}H${r.right - k}${arc(r.right, r.top + k)}` +
			`V${r.bottom - k}${arc(r.right - k, r.bottom)}H${r.left + k}${arc(r.left, r.bottom - k)}` +
			`V${r.top + k}${arc(r.left + k, r.top)}Z")`;
	}
	$effect(() => {
		if (!open || !spotlight) return;
		measure();
		window.addEventListener('scroll', measure, true);
		window.addEventListener('resize', measure);
		return () => {
			window.removeEventListener('scroll', measure, true);
			window.removeEventListener('resize', measure);
		};
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Portal>
		<Dialog.Overlay forceMount>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed inset-0 z-40 bg-overlay"
						style:clip-path={hole}
						transition:fade={{ duration: fadeDuration }}
					></div>
				{/if}
			{/snippet}
		</Dialog.Overlay>
		<Dialog.Content forceMount preventScroll={false} {onOpenAutoFocus} {onCloseAutoFocus}>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed inset-x-0 bottom-0 z-50 mx-auto flex {tall
							? 'max-h-[92dvh]'
							: 'max-h-[80dvh]'} w-full max-w-[640px] flex-col rounded-t-3xl bg-background shadow-sheet outline-none"
						transition:fly={{ duration, y: 200 }}
					>
						<SheetHandle label={closeLabel} onclose={() => (open = false)} />
						{@render children()}
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
