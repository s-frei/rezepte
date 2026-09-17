<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { fade, scale } from 'svelte/transition';
	import { m } from '$lib/paraglide/messages';
	import Button from './Button.svelte';

	let {
		open = $bindable(false),
		title,
		text,
		confirmLabel,
		destructive = false,
		onconfirm
	}: {
		open?: boolean;
		title: string;
		text: string;
		confirmLabel: string;
		destructive?: boolean;
		onconfirm: () => void;
	} = $props();

	function confirm() {
		open = false;
		onconfirm();
	}
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
						<Dialog.Title class="font-display text-heading font-medium">{title}</Dialog.Title>
						<Dialog.Description class="mt-2 text-body text-text-muted">{text}</Dialog.Description>
						<div class="mt-6 flex justify-end gap-3">
							<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
							<Button variant={destructive ? 'destructive' : 'primary'} onclick={confirm}>
								{confirmLabel}
							</Button>
						</div>
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
