<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { isSignedOut } from '$lib/api/client';
	import { clearSignInSettings, getSignInSettings, type SignInSettings } from '$lib/api/oidc';
	import CopyIcon from '$lib/components/icons/CopyIcon.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import { m } from '$lib/paraglide/messages';
	import { copyText } from '$lib/recipe/share.svelte';
	import { connectedWarning, providerName } from '$lib/settings/sign-in';
	import Letterhead, { urlSegments, type LetterheadLine } from './Letterhead.svelte';
	import OidcSettingsDialog from './OidcSettingsDialog.svelte';

	let current = $state<SignInSettings | null>(null);
	let loadFailed = $state(false);
	let editOpen = $state(false);
	let turnOffOpen = $state(false);
	let copyIcon = $state<CopyIcon>();

	onMount(async () => {
		try {
			current = await getSignInSettings();
		} catch (error) {
			loadFailed = !isSignedOut(error);
		}
	});

	const blank = $derived(!current || current.source === 'none');
	const lines = $derived<LetterheadLine[]>(
		current && !blank
			? [
					{
						key: m.settings_sign_in_line_provider(),
						main: providerName(current),
						sub: current.issuer,
						url: true
					},
					{
						key: m.settings_sign_in_line_client(),
						main: current.clientId,
						sub: current.secretSet
							? m.settings_sign_in_secret_set()
							: m.settings_sign_in_public_client()
					}
				]
			: [m.settings_sign_in_line_provider(), m.settings_sign_in_line_client()].map((key) => ({
					key,
					main: '—'
				}))
	);

	async function turnOff() {
		try {
			await clearSignInSettings();
			current = await getSignInSettings();
			toast.success(m.settings_sign_in_turned_off());
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.settings_sign_in_save_error());
		}
	}

	async function copy(uri: string) {
		if (await copyText(uri, m.settings_sign_in_redirect_copied())) copyIcon?.play();
	}
</script>

<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-sign-in">
	<h2 id="settings-sign-in" class="mb-4 font-display text-heading font-medium">
		{m.settings_nav_sign_in()}
	</h2>
	{#if loadFailed}
		<p class="text-body-sm text-text-muted">{m.settings_sign_in_load_error()}</p>
	{:else if !current}
		<Skeleton class="mt-6 h-32 rounded-lg" />
	{:else if current.publicUrlMissing && current.source === 'none'}
		<p class="max-w-[60ch] text-body-sm text-text-muted">
			{m.settings_sign_in_public_url_missing()}
		</p>
	{:else}
		{#if current.publicUrlMissing}
			<!-- A stored provider outlives the public URL; it stays visible and can be turned off. -->
			<p class="mb-4 max-w-[60ch] text-body-sm text-text-muted">
				{m.settings_sign_in_public_url_missing()}
			</p>
		{/if}
		<Letterhead
			{lines}
			{blank}
			stamp={{
				label:
					current.source === 'settings'
						? m.settings_sign_in_stamp_on()
						: current.source === 'env'
							? m.settings_sign_in_stamp_env()
							: m.settings_sign_in_stamp_off(),
				on: current.source === 'settings'
			}}
		/>
		{#if current.source === 'env'}
			<p class="mt-3 text-caption text-text-muted">{m.settings_sign_in_env_hint()}</p>
		{:else if current.source === 'none'}
			<p class="mt-3 text-body-sm text-text-muted">{m.settings_sign_in_off_hint()}</p>
			<div class="mt-4">
				<Button onclick={() => (editOpen = true)}>{m.settings_sign_in_set_up()}</Button>
			</div>
		{:else}
			<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
				{#if !current.publicUrlMissing}
					<Button variant="secondary" onclick={() => (editOpen = true)}
						>{m.settings_sign_in_edit()}</Button
					>
				{/if}
				<!-- Nobody connected, nobody to warn: it turns off at once. -->
				<Button
					variant="ghost"
					onclick={() => (current!.connected > 0 ? (turnOffOpen = true) : void turnOff())}
					>{m.settings_sign_in_turn_off()}</Button
				>
			</div>
		{/if}
		<!-- In every state: the address is registered at the provider before
		     there is anything to fill in here. -->
		{#if !current.publicUrlMissing}
			<div class="mt-5 border-t border-border pt-5">
				<p class="text-caption font-semibold">{m.settings_sign_in_redirect_label()}</p>
				<div class="mt-2 flex flex-wrap items-center gap-2.5">
					<code
						class="min-w-0 flex-1 basis-60 rounded-md border border-border bg-background px-4 py-2.5 font-mono text-body-sm wrap-anywhere"
						>{#each urlSegments(current.redirectUri) as segment, i (i)}{segment}<wbr />{/each}</code
					>
					<Button
						variant="secondary"
						label={m.settings_sign_in_redirect_copy()}
						onclick={() => void copy(current!.redirectUri)}
					>
						<CopyIcon bind:this={copyIcon} class="size-4" />
						{m.access_copy()}
					</Button>
				</div>
				<p class="mt-2 text-micro text-text-muted">{m.settings_sign_in_redirect_hint()}</p>
			</div>
		{/if}
	{/if}
</section>

{#if current}
	<OidcSettingsDialog bind:open={editOpen} {current} onsaved={(next) => (current = next)} />
	<ConfirmDialog
		bind:open={turnOffOpen}
		title={m.settings_sign_in_turn_off_title({ name: providerName(current) })}
		text={connectedWarning(current)}
		confirmLabel={m.settings_sign_in_turn_off()}
		destructive
		onconfirm={() => void turnOff()}
	/>
{/if}
