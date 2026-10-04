<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import Plus from '@lucide/svelte/icons/plus';
	import { toast } from 'svelte-sonner';
	import { isSignedOut } from '$lib/api/client';
	import {
		deleteToken,
		listTokens,
		type ApiToken,
		type CreatedApiToken,
		type TokenScope
	} from '$lib/api/tokens';
	import ApiAccessCard from '$lib/components/settings/ApiAccessCard.svelte';
	import CreateTokenDialog from '$lib/components/settings/CreateTokenDialog.svelte';
	import SettingsLayout from '$lib/components/settings/SettingsLayout.svelte';
	import TokenRevealDialog from '$lib/components/settings/TokenRevealDialog.svelte';
	import TokenRow from '$lib/components/settings/TokenRow.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import TagChip from '$lib/components/ui/TagChip.svelte';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import { tokenSupply } from '$lib/settings/token-access';
	import { userColorClasses } from '$lib/user/color';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	let tokens = $state<ApiToken[]>([]);
	let loading = $state(true);
	let loadFailed = $state(false);
	let createOpen = $state(false);
	let revealOpen = $state(false);
	let revealed = $state('');
	let revealedScopes = $state<TokenScope[]>([]);
	let revokeOpen = $state(false);
	let revokeTarget = $state<ApiToken | null>(null);

	// The list shows the viewer's own tokens unless they switch to everyone's,
	// and then may narrow it to one issuer - the shared-links page's filter,
	// kept in the URL the same way (?all=1&user=<id>). The server already
	// sends every token to an admin; both are view filters, not settings.
	let everyone = $state(page.url.searchParams.get('all') === '1');
	let person = $state(page.url.searchParams.get('user') ?? '');

	const own = $derived(tokens.filter((t) => t.ownerId === data.userId));
	// A person from the URL who holds no token (any more) filters nothing.
	const selectedPerson = $derived(tokens.some((t) => t.ownerId === person) ? person : '');
	const visible = $derived(
		!everyone ? own : selectedPerson ? tokens.filter((t) => t.ownerId === selectedPerson) : tokens
	);
	const issuers = $derived.by(() => {
		const list: { id: string; displayName: string; color: string; count: number }[] = [];
		for (const t of tokens) {
			const entry = list.find((i) => i.id === t.ownerId);
			if (entry) entry.count++;
			else
				list.push({
					id: t.ownerId,
					displayName: t.ownerDisplayName,
					color: t.ownerColor,
					count: 1
				});
		}
		return list.sort((a, b) => a.displayName.localeCompare(b.displayName, getLocale()));
	});

	function syncUrl() {
		const params: string[] = [];
		if (everyone) {
			params.push('all=1');
			if (person) params.push(`user=${encodeURIComponent(person)}`);
		}
		const qs = params.join('&');
		void goto(resolve(qs ? `/settings/api?${qs}` : '/settings/api'), {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}

	function toggleEveryone(on: boolean) {
		everyone = on;
		if (!on) person = '';
		syncUrl();
	}

	function pickPerson(id: string) {
		person = selectedPerson === id ? '' : id;
		syncUrl();
	}

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

	// Step one of the access card is about the viewer's own token, so the
	// household's tokens do not count towards it.
	const supply = $derived(loading || loadFailed ? undefined : tokenSupply(own));

	// The raw secret must not outlive the dialog that shows it once.
	$effect(() => {
		if (!revealOpen) {
			revealed = '';
		}
	});

	function created(token: CreatedApiToken) {
		revealed = token.token;
		revealedScopes = token.scopes;
		revealOpen = true;
		// Named fields only, not a rest spread: the secret must not linger in
		// the list's state one moment longer than the reveal dialog needs it.
		const {
			id,
			name,
			prefix,
			scopes,
			ownerId,
			ownerUsername,
			ownerDisplayName,
			ownerColor,
			createdAt,
			expiresAt,
			lastUsedAt
		} = token;
		tokens = [
			{
				id,
				name,
				prefix,
				scopes,
				ownerId,
				ownerUsername,
				ownerDisplayName,
				ownerColor,
				createdAt,
				expiresAt,
				lastUsedAt
			},
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
	<ApiAccessCard isAdmin={data.isAdmin} loading={data.isAdmin && loading} {supply} />

	<!-- Always there for an admin, empty or not, so "Create token" has one
		 fixed home; the card above points down here. -->
	{#if data.isAdmin}
		<section class="rounded-2xl bg-surface p-6 md:p-8" aria-labelledby="settings-tokens">
			<div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
				<h2 id="settings-tokens" class="font-display text-heading font-medium">
					{m.tokens_title()}
				</h2>
				<Button class="whitespace-nowrap" onclick={() => (createOpen = true)}>
					<Plus aria-hidden="true" class="size-4" />
					{m.tokens_create()}
				</Button>
			</div>

			<div
				class="mt-4 mb-4 flex items-center justify-between gap-3 rounded-md bg-background px-4 py-3"
			>
				<span class="text-body-sm font-medium">{m.tokens_everyone()}</span>
				<Switch checked={everyone} label={m.tokens_everyone()} onchange={toggleEveryone} />
			</div>
			<!-- Hidden below two issuers, like the shared links' person filter:
				 with one there is nothing to pick between. -->
			{#if everyone && !loading && issuers.length > 1}
				<section aria-labelledby="settings-tokens-person" class="mb-4 space-y-2">
					<h3 id="settings-tokens-person" class="text-caption font-semibold text-text-muted">
						{m.tokens_person_heading()}
					</h3>
					<div class="flex flex-wrap gap-2">
						{#each issuers as issuer (issuer.id)}
							<TagChip
								label={issuer.displayName}
								count={issuer.count}
								active={selectedPerson === issuer.id}
								onclick={() => pickPerson(issuer.id)}
							>
								{#snippet leading()}
									<span
										aria-hidden="true"
										class="size-3 rounded-full {userColorClasses(issuer.color)}"
									></span>
								{/snippet}
							</TagChip>
						{/each}
					</div>
				</section>
			{/if}

			{#if loading}
				<Skeleton class="h-16 w-full rounded-lg" />
			{:else if loadFailed}
				<div class="flex flex-col items-start gap-3">
					<p class="text-body-sm text-text-muted">{m.tokens_load_error()}</p>
					<Button variant="ghost" onclick={() => void load()}>{m.common_retry()}</Button>
				</div>
			{:else if visible.length === 0}
				<p
					class="rounded-lg border border-dashed border-border px-4 py-5 text-body-sm text-text-muted"
				>
					{m.tokens_empty()}
				</p>
			{:else}
				<ul aria-label={m.tokens_title()} class="grid grid-cols-[minmax(0,1fr)] gap-3.5">
					{#each visible as token (token.id)}
						<TokenRow {token} showIssuer={everyone} onrevoke={askRevoke} />
					{/each}
				</ul>
			{/if}
		</section>
	{/if}
</SettingsLayout>

{#if data.isAdmin}
	<CreateTokenDialog bind:open={createOpen} oncreated={created} />
	<TokenRevealDialog bind:open={revealOpen} token={revealed} scopes={revealedScopes} />
	<ConfirmDialog
		bind:open={revokeOpen}
		title={m.tokens_revoke_title()}
		text={m.tokens_revoke_text({ name: revokeTarget?.name ?? '' })}
		confirmLabel={m.tokens_revoke()}
		destructive
		onconfirm={() => void revoke()}
	/>
{/if}
