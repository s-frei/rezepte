<script lang="ts">
	import { Dialog } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade, fly } from 'svelte/transition';

	// Shared Overlay + Content wrapper for the phone's bottom sheets (the
	// contents, You, one person), so the phone has one kind of sheet: a
	// rounded panel rising from the bottom edge, its handle the close button.
	// The caller renders the title and the scrolling body inside it.
	let {
		open = $bindable(false),
		closeLabel,
		onOpenAutoFocus,
		onCloseAutoFocus,
		children
	}: {
		open?: boolean;
		/** Accessible name of the handle, which closes the sheet. */
		closeLabel: string;
		/** Forwarded to Bits UI: where focus goes once the sheet has opened. */
		onOpenAutoFocus?: (event: Event) => void;
		/** Forwarded to Bits UI: where focus goes once the sheet has closed. */
		onCloseAutoFocus?: (event: Event) => void;
		children: Snippet;
	} = $props();

	const duration = $derived(prefersReducedMotion.current ? 0 : 250);
	const fadeDuration = $derived(prefersReducedMotion.current ? 0 : 200);
</script>

<Dialog.Root bind:open>
	<Dialog.Portal>
		<Dialog.Overlay forceMount>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed inset-0 z-40 bg-overlay"
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
						class="fixed inset-x-0 bottom-0 z-50 mx-auto flex max-h-[80dvh] w-full max-w-[640px] flex-col rounded-t-3xl bg-background shadow-sheet outline-none"
						transition:fly={{ duration, y: 200 }}
					>
						<Dialog.Close
							aria-label={closeLabel}
							class="flex w-full shrink-0 justify-center pt-2 pb-3 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
						>
							<span class="h-1 w-9 rounded-pill bg-handle" aria-hidden="true"></span>
						</Dialog.Close>
						{@render children()}
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
