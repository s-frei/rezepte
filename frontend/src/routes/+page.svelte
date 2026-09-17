<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { Button } from 'bits-ui';
	import { logout } from '$lib/api/auth';
	import { session } from '$lib/auth.svelte';
	import { m } from '$lib/paraglide/messages';

	async function signOut() {
		try {
			await logout();
		} catch {
			// The server call failed (e.g. the session was already gone) - the
			// user still expects to end up signed out and on the login page.
		} finally {
			session.user = null;
			await invalidateAll();
			await goto(resolve('/login'));
		}
	}
</script>

<main class="mx-auto max-w-3xl p-8">
	<div class="flex items-center justify-between">
		<h1 class="font-display text-display-lg font-semibold">{m.app_name()}</h1>
		{#if session.user}
			<div class="flex items-center gap-3 text-body-sm">
				<span class="text-text-muted"
					>{m.home_signed_in_as({ username: session.user.username })}</span
				>
				<Button.Root
					onclick={signOut}
					class="h-10 rounded-pill border border-border bg-surface px-4 font-semibold transition hover:brightness-95 active:scale-[.98]"
				>
					{m.logout()}
				</Button.Root>
			</div>
		{/if}
	</div>
	<p class="mt-4 text-body text-text-muted">{m.home_placeholder()}</p>
</main>
