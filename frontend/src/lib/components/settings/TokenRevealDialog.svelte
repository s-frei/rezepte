<script lang="ts">
	import { Dialog, Tabs } from 'bits-ui';
	import Copy from 'lucide-svelte/icons/copy';
	import { toast } from 'svelte-sonner';
	import type { TokenScope } from '$lib/api/tokens';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';
	import { claudeCodeSnippet, jsonSnippet, mcpUrl, offersMcp } from '$lib/settings/mcp';

	let {
		open = $bindable(false),
		token,
		scopes = []
	}: { open?: boolean; token: string; scopes?: TokenScope[] } = $props();

	// Read off the browser for the same reason ApiReferenceCard does: the
	// instance is wherever the reverse proxy in front of it put it.
	const url = $derived(typeof window === 'undefined' ? '/mcp' : mcpUrl(window.location.origin));
	const claudeSnippet = $derived(claudeCodeSnippet(url, token));
	const jsonSnippetText = $derived(jsonSnippet(url, token));

	async function copy() {
		await navigator.clipboard.writeText(token);
		toast.success(m.tokens_reveal_copied());
	}

	async function copySnippet(value: string) {
		await navigator.clipboard.writeText(value);
		toast.success(m.tokens_mcp_copied());
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

	{#if offersMcp(scopes)}
		<section class="mt-6" aria-labelledby="token-mcp">
			<h3 id="token-mcp" class="text-body-sm font-semibold">{m.tokens_mcp_title()}</h3>
			<p class="mt-1 text-caption text-text-muted">{m.tokens_mcp_intro()}</p>
			<Tabs.Root value="claude" class="mt-3">
				<Tabs.List class="flex gap-1 rounded-pill bg-background p-1">
					<Tabs.Trigger
						value="claude"
						class="flex-1 rounded-pill px-3 py-1.5 text-caption font-semibold text-text-muted transition data-[state=active]:bg-surface data-[state=active]:text-text data-[state=active]:shadow-card"
					>
						{m.tokens_mcp_tab_claude()}
					</Tabs.Trigger>
					<Tabs.Trigger
						value="json"
						class="flex-1 rounded-pill px-3 py-1.5 text-caption font-semibold text-text-muted transition data-[state=active]:bg-surface data-[state=active]:text-text data-[state=active]:shadow-card"
					>
						{m.tokens_mcp_tab_json()}
					</Tabs.Trigger>
				</Tabs.List>
				<Tabs.Content
					value="claude"
					class="mt-2 flex items-start gap-2 rounded-md bg-surface-elevated p-3"
				>
					<pre
						class="min-w-0 flex-1 overflow-x-auto font-mono text-caption break-all whitespace-pre-wrap">{claudeSnippet}</pre>
					<Button variant="ghost" onclick={() => void copySnippet(claudeSnippet)}>
						<Copy aria-hidden="true" class="size-4" />
						{m.tokens_reveal_copy()}
					</Button>
				</Tabs.Content>
				<Tabs.Content
					value="json"
					class="mt-2 flex items-start gap-2 rounded-md bg-surface-elevated p-3"
				>
					<pre
						class="min-w-0 flex-1 overflow-x-auto font-mono text-caption break-all whitespace-pre-wrap">{jsonSnippetText}</pre>
					<Button variant="ghost" onclick={() => void copySnippet(jsonSnippetText)}>
						<Copy aria-hidden="true" class="size-4" />
						{m.tokens_reveal_copy()}
					</Button>
				</Tabs.Content>
			</Tabs.Root>
		</section>
	{/if}

	<div class="mt-6 flex justify-end">
		<Button onclick={() => (open = false)}>{m.tokens_reveal_done()}</Button>
	</div>
</BaseDialog>
