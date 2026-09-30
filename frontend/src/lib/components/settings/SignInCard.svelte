<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import { getOwnIdentity, unlinkOwnIdentity } from '$lib/api/oidc';
	import { session } from '$lib/auth.svelte';
	import ProviderButton from '$lib/components/auth/ProviderButton.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/recipe/format';

	let { name }: { name: string } = $props();

	let linkedAt = $state<string | null>(null);
	let loaded = $state(false);
	let busy = $state(false);
	// Disconnecting an account without a password would lock its owner out;
	// the service refuses it too (409), this only says so up front.
	const hasPassword = $derived(!!session.user?.hasPassword);

	onMount(() => {
		// The provider flow comes back to /settings?oidc=...: say how it went
		// once, then drop the query so a reload does not say it again.
		const result = page.url.searchParams.get('oidc');
		if (result === 'linked') toast.success(m.oidc_linked({ name }));
		else if (result === 'taken') toast.error(m.oidc_taken({ name }));
		else if (result === 'failed') toast.error(m.oidc_failed({ name }));
		if (result) {
			void goto(resolve('/settings'), { replaceState: true, keepFocus: true, noScroll: true });
		}
		void load();
	});

	async function load() {
		try {
			linkedAt = (await getOwnIdentity())?.linkedAt ?? null;
		} catch {
			// Unknown reads as not connected: connecting an identity the
			// account already has is harmless, the service treats it as done.
			linkedAt = null;
		} finally {
			loaded = true;
		}
	}

	async function disconnect() {
		busy = true;
		try {
			await unlinkOwnIdentity();
			linkedAt = null;
		} catch (e) {
			if (e instanceof ApiError && e.status === 409) {
				toast.error(m.settings_signin_need_password());
			} else if (!isSignedOut(e)) {
				toast.error(m.users_update_error());
			}
		} finally {
			busy = false;
		}
	}
</script>

{#if loaded}
	{#if linkedAt}
		<p class="text-body">{m.settings_signin_linked({ name, date: formatDate(linkedAt) })}</p>
		<div class="mt-4 space-y-2">
			<Button variant="secondary" disabled={busy || !hasPassword} onclick={disconnect}>
				{m.settings_signin_disconnect()}
			</Button>
			{#if !hasPassword}
				<p class="text-caption text-text-muted">{m.settings_signin_need_password()}</p>
			{/if}
		</div>
	{:else}
		<p class="mb-4 text-body text-text-muted">{m.settings_signin_unlinked({ name })}</p>
		<ProviderButton intent="link" {name} />
	{/if}
{/if}
