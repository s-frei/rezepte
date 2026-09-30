<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { fade, scale } from 'svelte/transition';
	import type { Snippet } from 'svelte';

	// Shared Overlay + Content wrapper for the app's small centered dialogs
	// (confirm, create/reset user). The command palette has its own layout
	// (top-anchored, wider, different padding) and isn't a caller of this.
	let {
		open = $bindable(false),
		dismissible = true,
		wide = false,
		children
	}: {
		open?: boolean;
		/**
		 * False keeps the dialog open on Escape and on a click outside, so it
		 * can only be closed through its own button. The token reveal needs
		 * this: the secret is shown once, and a stray Escape would lose it.
		 */
		dismissible?: boolean;
		/** 720px instead of 440px: the pass-on dialog sets its card beside its actions. */
		wide?: boolean;
		children: Snippet;
	} = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Portal>
		<Dialog.Overlay forceMount>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed inset-0 z-40 bg-overlay"
						transition:fade={{ duration: 200 }}
					></div>
				{/if}
			{/snippet}
		</Dialog.Overlay>
		<Dialog.Content
			forceMount
			preventScroll={false}
			escapeKeydownBehavior={dismissible ? 'close' : 'ignore'}
			interactOutsideBehavior={dismissible ? 'close' : 'ignore'}
		>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<!-- Capped at the dynamic viewport and scrolling inside itself: the
					     create-user form is taller than a phone screen, and a centered
					     box without a cap pushes its title and its buttons off both
					     edges where nothing can scroll them back. `dvh` rather than
					     `vh` so the cap follows the browser bar and the keyboard. -->
					<div
						{...props}
						class="fixed top-1/2 left-1/2 z-50 max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] {wide
							? 'max-w-[720px]'
							: 'max-w-[440px]'} -translate-x-1/2 -translate-y-1/2 overflow-y-auto overscroll-contain rounded-3xl bg-surface p-5 shadow-dialog md:p-7"
						transition:scale={{ duration: 200, start: 0.95 }}
					>
						{@render children()}
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
