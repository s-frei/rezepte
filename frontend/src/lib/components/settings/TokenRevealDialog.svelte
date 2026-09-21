<script lang="ts">
	import { Dialog } from 'bits-ui';
	import Copy from 'lucide-svelte/icons/copy';
	import { toast } from 'svelte-sonner';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';

	let { open = $bindable(false), token }: { open?: boolean; token: string } = $props();

	async function copy() {
		await navigator.clipboard.writeText(token);
		toast.success(m.tokens_reveal_copied());
	}
</script>

<!-- dismissible={false}: the secret is shown once, so only the explicit
	 button may close this. -->
<BaseDialog bind:open dismissible={false}>
	<Dialog.Title class="font-display text-heading font-medium">
		{m.tokens_reveal_title()}
	</Dialog.Title>
	<Dialog.Description class="mt-2 text-body text-text-muted">
		{m.tokens_reveal_warning()}
	</Dialog.Description>
	<div class="mt-5 flex items-center gap-2 rounded-md bg-surface-elevated p-3">
		<code class="min-w-0 flex-1 font-mono text-body-sm break-all">{token}</code>
		<Button variant="ghost" onclick={() => void copy()}>
			<Copy aria-hidden="true" class="size-4" />
			{m.tokens_reveal_copy()}
		</Button>
	</div>
	<div class="mt-6 flex justify-end">
		<Button onclick={() => (open = false)}>{m.tokens_reveal_done()}</Button>
	</div>
</BaseDialog>
