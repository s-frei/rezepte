<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { scale } from 'svelte/transition';
	import { prefersReducedMotion } from 'svelte/motion';
	import type { Snippet } from 'svelte';
	import Overlay from './Overlay.svelte';

	// Shared Overlay + Content wrapper for the app's small centered dialogs
	// (confirm, create/reset user). The command palette has its own layout
	// (top-anchored, wider, different padding) and isn't a caller of this.
	let {
		open = $bindable(false),
		dismissible = true,
		wide = false,
		bleed = false,
		onCloseAutoFocus,
		children
	}: {
		open?: boolean;
		/**
		 * False keeps the dialog open on Escape and on a click outside, so it
		 * can only be closed through its own button. The token reveal needs
		 * this: the secret is shown once, and a stray Escape would lose it.
		 */
		dismissible?: boolean;
		/**
		 * 720px instead of 440px: the pass-on dialog sets its card beside its
		 * actions, the add-account form its two columns side by side.
		 */
		wide?: boolean;
		/**
		 * All of the padding inside the scroller, none around it, so content
		 * can bleed to the rounded edge with `-m-7` (the pass-on dialog's
		 * preview panel). The price: a scrollbar, should one appear, runs
		 * along the edge instead of inside it.
		 */
		bleed?: boolean;
		/** Forwarded to Bits UI: where focus goes once the dialog has closed. */
		onCloseAutoFocus?: (event: Event) => void;
		children: Snippet;
	} = $props();

	const duration = $derived(prefersReducedMotion.current ? 0 : 200);
</script>

<Dialog.Root bind:open>
	<Dialog.Portal>
		<Overlay />
		<Dialog.Content
			forceMount
			preventScroll={false}
			escapeKeydownBehavior={dismissible ? 'close' : 'ignore'}
			interactOutsideBehavior={dismissible ? 'close' : 'ignore'}
			{onCloseAutoFocus}
		>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<!-- Capped at the dynamic viewport and scrolling inside itself: the
					     create-user form is taller than a phone screen, and a centered
					     box without a cap pushes its title and its buttons off both
					     edges where nothing can scroll them back. `dvh` rather than
					     `vh` so the cap follows the browser bar and the keyboard.
					     A word longer than the line ("Einrichtungslink" at 200% text on
					     a phone) breaks instead of scrolling the dialog sideways; the
					     title (Dialog.Title is an h2) hyphenates, a button's label
					     never does ("Ko-pie-ren").
					     The rounded box clips and its contents scroll inside its
					     padding, so a scrollbar runs on the straight part of the edge
					     (as SearchBar's list): 12px down the 24px corner curves 3px
					     in, which the 6px side clears. The padding is split between
					     the two, 20px (28px from md) in all. -->
					<div
						{...props}
						class="fixed top-1/2 left-1/2 z-50 flex max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] {wide
							? 'max-w-[720px]'
							: 'max-w-[440px]'} -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-3xl bg-surface shadow-dialog {bleed
							? ''
							: 'px-1.5 py-3'}"
						transition:scale={{ duration, start: 0.95 }}
					>
						<div
							class="min-h-0 overflow-y-auto overscroll-contain wrap-break-word [&_h2]:hyphens-auto {bleed
								? 'p-7'
								: 'px-3.5 py-2 md:px-[22px] md:py-4'}"
						>
							{@render children()}
						</div>
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
