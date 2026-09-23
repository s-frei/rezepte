<script lang="ts">
	import '../app.css';
	import type { Snippet } from 'svelte';
	import { Toaster } from 'svelte-sonner';
	import { page } from '$app/state';
	import AppShell from '$lib/components/shell/AppShell.svelte';
	import CommandPalette from '$lib/components/ui/CommandPalette.svelte';
	import { session } from '$lib/auth.svelte';
	import { palette } from '$lib/palette.svelte';
	import { getLocale } from '$lib/paraglide/runtime';

	let { children }: { children: Snippet } = $props();

	const isLoginRoute = $derived(page.url.pathname.startsWith('/login'));
	// Full-screen routes render without the app shell: no top bar, no bottom
	// nav, no command palette and no Cmd+K. Matched on the route id rather
	// than the pathname so the check is exact and independent of the slug.
	const isFullscreenRoute = $derived(page.route.id === '/recipes/[slug]/cook');
	const bare = $derived(isLoginRoute || isFullscreenRoute);

	// app.html ships a static lang attribute; the real locale is only known
	// once Paraglide has resolved it. Screen readers and spell checking read
	// this, so it has to agree with what is on screen.
	//
	// getLocale() reads the cookie, which Svelte cannot track. Login and
	// sign-out rewrite that cookie without a reload, and both set session.user
	// right after, so reading it here re-runs the effect at exactly those two
	// points. Every other locale change reloads the page.
	$effect(() => {
		void session.user;
		document.documentElement.lang = getLocale();
	});

	function handleKeydown(event: KeyboardEvent) {
		if (bare) {
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

<!-- `min-h-dvh`, not `min-h-screen`: on mobile browsers `100vh` is taller than
     the visible viewport, so the wrapper would outgrow the `h-dvh` cooking
     page and the document would scroll under a full-screen route. -->
<div class="min-h-dvh bg-background font-sans text-text">
	{#if bare}
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
				'rounded-toast border-none bg-inverse px-[18px] py-3.5 text-body-sm font-semibold text-inverse-foreground shadow-dialog',
			description: 'text-inverse-muted',
			actionButton: 'bg-transparent font-medium text-inverse-muted',
			cancelButton: 'bg-transparent font-medium text-inverse-muted',
			closeButton: 'border-border bg-inverse text-inverse-foreground'
		}
	}}
/>
