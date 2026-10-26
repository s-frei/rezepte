<script lang="ts">
	import { Dialog } from 'bits-ui';
	import type { Snippet } from 'svelte';
	import { m } from '$lib/paraglide/messages';
	import BaseDialog from './BaseDialog.svelte';
	import Button from './Button.svelte';

	let {
		open = $bindable(false),
		title,
		text,
		confirmLabel,
		destructive = false,
		onconfirm,
		children
	}: {
		open?: boolean;
		title: string;
		text: string;
		confirmLabel: string;
		destructive?: boolean;
		onconfirm: () => void;
		/** Shown under the text: what the confirmation is about, in detail. */
		children?: Snippet;
	} = $props();

	function confirm() {
		open = false;
		onconfirm();
	}
</script>

<BaseDialog bind:open>
	<Dialog.Title class="font-display text-heading font-medium">{title}</Dialog.Title>
	<Dialog.Description class="mt-2 text-body text-text-muted">{text}</Dialog.Description>
	{@render children?.()}
	<div class="mt-6 flex justify-end gap-3">
		<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
		<Button variant={destructive ? 'destructive' : 'primary'} onclick={confirm}>
			{confirmLabel}
		</Button>
	</div>
</BaseDialog>
