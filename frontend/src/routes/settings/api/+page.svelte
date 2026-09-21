<script lang="ts">
	import { onMount } from 'svelte';
	import Plus from 'lucide-svelte/icons/plus';
	import { toast } from 'svelte-sonner';
	import { isSignedOut } from '$lib/api/client';
	import { deleteToken, listTokens, type ApiToken, type CreatedApiToken } from '$lib/api/tokens';
	import ApiReferenceCard from '$lib/components/settings/ApiReferenceCard.svelte';
	import CreateTokenDialog from '$lib/components/settings/CreateTokenDialog.svelte';
	import SettingsLayout from '$lib/components/settings/SettingsLayout.svelte';
	import TokenRevealDialog from '$lib/components/settings/TokenRevealDialog.svelte';
	import TokenTable from '$lib/components/settings/TokenTable.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	let tokens = $state<ApiToken[]>([]);
	let loading = $state(true);
	let loadFailed = $state(false);
	let createOpen = $state(false);
	let revealOpen = $state(false);
	let revealed = $state('');
	let revokeOpen = $state(false);
	let revokeTarget = $state<ApiToken | null>(null);

	// An inline error with a retry button instead of a toast over an empty
	// list: an admin cannot tell a failed load from "no tokens", and a toast
	// leaves no way back onto the list short of reloading the page.
	async function load() {
		loading = true;
		loadFailed = false;
		try {
			tokens = await listTokens();
		} catch (error) {
			// On a 401 the client is already navigating to the login page.
			loadFailed = !isSignedOut(error);
		} finally {
			loading = false;
		}
	}

	// A member has no business calling GET /tokens: it answers 403 and the page
	// would show its load-error state instead of its reference card.
	onMount(() => {
		if (data.isAdmin) {
			void load();
		}
	});

	// The raw secret must not outlive the dialog that shows it once.
	$effect(() => {
		if (!revealOpen) {
			revealed = '';
		}
	});

	function created(token: CreatedApiToken) {
		revealed = token.token;
		revealOpen = true;
		// Named fields only, not a rest spread: the secret must not linger in
		// the list's state one moment longer than the reveal dialog needs it.
		const { id, name, prefix, scopes, ownerId, ownerName, createdAt, expiresAt, lastUsedAt } =
			token;
		tokens = [
			{ id, name, prefix, scopes, ownerId, ownerName, createdAt, expiresAt, lastUsedAt },
			...tokens
		];
	}

	function askRevoke(token: ApiToken) {
		revokeTarget = token;
		revokeOpen = true;
	}

	async function revoke() {
		const target = revokeTarget;
		if (target === null) {
			return;
		}
		try {
			await deleteToken(target.id);
			tokens = tokens.filter((t) => t.id !== target.id);
			toast.success(m.tokens_revoked({ name: target.name }));
		} catch (error) {
			if (!isSignedOut(error)) {
				toast.error(m.tokens_revoke_error());
			}
		}
	}
</script>

<svelte:head><title>{m.settings_nav_api()} · {m.app_name()}</title></svelte:head>

<SettingsLayout active="api">
	<ApiReferenceCard showTokenHint={!data.isAdmin} />

	{#if data.isAdmin}
		<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-tokens">
			<div class="mb-4 flex items-start justify-between gap-4">
				<div>
					<h2 id="settings-tokens" class="font-display text-heading font-medium">
						{m.tokens_title()}
					</h2>
					<p class="mt-1 text-caption text-text-muted">{m.tokens_intro()}</p>
				</div>
				<Button onclick={() => (createOpen = true)}>
					<Plus aria-hidden="true" class="size-4" />
					{m.tokens_create()}
				</Button>
			</div>

			{#if loading}
				<Skeleton class="h-24 w-full" />
			{:else if loadFailed}
				<div class="flex flex-col items-start gap-3">
					<p class="text-body-sm text-text-muted">{m.tokens_load_error()}</p>
					<Button variant="ghost" onclick={() => void load()}>{m.common_retry()}</Button>
				</div>
			{:else if tokens.length === 0}
				<EmptyState title={m.tokens_empty_title()} text={m.tokens_empty_text()}>
					<Button onclick={() => (createOpen = true)}>
						<Plus aria-hidden="true" class="size-4" />
						{m.tokens_create()}
					</Button>
				</EmptyState>
			{:else}
				<TokenTable {tokens} onrevoke={askRevoke} />
			{/if}
		</section>
	{/if}
</SettingsLayout>

{#if data.isAdmin}
	<CreateTokenDialog bind:open={createOpen} oncreated={created} />
	<TokenRevealDialog bind:open={revealOpen} token={revealed} />
	<ConfirmDialog
		bind:open={revokeOpen}
		title={m.tokens_revoke_title()}
		text={m.tokens_revoke_text({ name: revokeTarget?.name ?? '' })}
		confirmLabel={m.tokens_revoke()}
		destructive
		onconfirm={() => void revoke()}
	/>
{/if}
