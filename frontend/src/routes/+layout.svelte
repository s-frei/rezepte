<script lang="ts">
	import '../app.css';
	import type { Snippet } from 'svelte';
	import { Toaster } from 'svelte-sonner';
	import { page } from '$app/state';
	import AppShell from '$lib/components/shell/AppShell.svelte';
	import CommandPalette from '$lib/components/ui/CommandPalette.svelte';
	import { palette } from '$lib/palette.svelte';

	let { children }: { children: Snippet } = $props();

	const isLoginRoute = $derived(page.url.pathname.startsWith('/login'));

	function handleKeydown(event: KeyboardEvent) {
		if (isLoginRoute) {
			return;
		}
		if (
			(event.metaKey || event.ctrlKey) &&
			!event.shiftKey &&
			!event.altKey &&
			event.key.toLowerCase() === 'k'
		) {
			event.preventDefault();
			palette.open = !palette.open;
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="min-h-screen bg-background font-sans text-text">
	{#if isLoginRoute}
		{@render children()}
	{:else}
		<AppShell>
			{@render children()}
		</AppShell>
		<CommandPalette />
	{/if}
</div>

<Toaster
	richColors={false}
	toastOptions={{
		classes: {
			toast:
				'rounded-2xl border-none bg-inverse px-4 py-3.5 text-body-sm font-semibold text-inverse-foreground shadow-dialog',
			description: 'text-inverse-muted',
			actionButton: 'bg-transparent font-medium text-inverse-muted',
			cancelButton: 'bg-transparent font-medium text-inverse-muted',
			closeButton: 'border-border bg-inverse text-inverse-foreground'
		}
	}}
/>
