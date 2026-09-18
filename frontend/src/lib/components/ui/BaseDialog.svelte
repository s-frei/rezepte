<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { fade, scale } from 'svelte/transition';
	import type { Snippet } from 'svelte';

	// Shared Overlay + Content wrapper for the app's small centred dialogs
	// (confirm, create/reset user). The command palette has its own layout
	// (top-anchored, wider, different padding) and isn't a caller of this.
	let {
		open = $bindable(false),
		children
	}: {
		open?: boolean;
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
		<Dialog.Content forceMount preventScroll={false}>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed top-1/2 left-1/2 z-50 w-[calc(100%-2.5rem)] max-w-[440px] -translate-x-1/2 -translate-y-1/2 rounded-3xl bg-surface p-7 shadow-dialog"
						transition:scale={{ duration: 200, start: 0.95 }}
					>
						{@render children()}
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
