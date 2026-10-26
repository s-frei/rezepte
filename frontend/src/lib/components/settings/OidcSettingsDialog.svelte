<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import { saveSignInSettings, type SignInConfig, type SignInSettings } from '$lib/api/oidc';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { m } from '$lib/paraglide/messages';
	import { connectedWarning, providerName, secretRequired } from '$lib/settings/sign-in';

	let {
		open = $bindable(false),
		current,
		onsaved
	}: {
		open?: boolean;
		current: SignInSettings;
		onsaved: (next: SignInSettings) => void;
	} = $props();

	let issuer = $state('');
	let name = $state('');
	let clientId = $state('');
	let secret = $state('');
	let removeSecret = $state(false);
	let issuerError = $state<string | null>(null);
	let clientIdError = $state<string | null>(null);
	let secretError = $state<string | null>(null);
	let saving = $state(false);
	let switchOpen = $state(false);

	// Refilled from what is stored every time the dialog opens, so a cancelled
	// edit never lingers into the next one.
	$effect(() => {
		if (!open) return;
		issuer = current.issuer;
		name = current.name;
		clientId = current.clientId;
		secret = '';
		removeSecret = false;
		issuerError = clientIdError = secretError = null;
	});

	function config(): SignInConfig {
		return {
			issuer: issuer.trim(),
			clientId: clientId.trim(),
			name: name.trim(),
			// Untouched means "keep": the field is never filled with the stored secret.
			...(secret || removeSecret || !current.secretSet ? { clientSecret: secret } : {})
		};
	}

	function submit(event: SubmitEvent) {
		event.preventDefault();
		issuerError = issuer.trim() ? null : m.settings_sign_in_field_required();
		clientIdError = clientId.trim() ? null : m.settings_sign_in_field_required();
		if (issuerError || clientIdError) return;
		// The server would refuse this; say so before the switch warning.
		if (
			secretRequired(current, {
				issuer: issuer.trim(),
				clientId: clientId.trim(),
				secret,
				removeSecret
			})
		) {
			secretError = m.settings_sign_in_field_secret_required();
			return;
		}
		// Another issuer leaves the connected accounts behind; say so first.
		const switching = current.source === 'settings' && issuer.trim() !== current.issuer;
		if (switching && current.connected > 0) switchOpen = true;
		else void save();
	}

	async function save() {
		saving = true;
		try {
			onsaved(await saveSignInSettings(config()));
			toast.success(m.settings_sign_in_saved());
			open = false;
		} catch (error) {
			if (isSignedOut(error)) return;
			const field = error instanceof ApiError && error.status === 422 ? error.errors[0] : undefined;
			if (field?.location === 'body.issuer') {
				issuerError = m.settings_sign_in_field_issuer_unreachable();
			} else if (field?.location === 'body.clientSecret') {
				secretError = m.settings_sign_in_field_secret_required();
			} else {
				toast.error(m.settings_sign_in_save_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

{#snippet section(title: string)}
	<!-- The add-account dialog's heading: italic display face in primary,
	     run out to the column's edge by a dotted leader. -->
	<div class="flex items-baseline gap-3">
		<h3 class="font-display text-body-lg font-medium text-primary italic">{title}</h3>
		<span aria-hidden="true" class="min-w-6 flex-1 border-b-2 border-dotted border-border"></span>
	</div>
{/snippet}

{#snippet secretState()}
	{#if removeSecret}
		{m.settings_sign_in_field_secret_removed()} ·
		<button type="button" class="text-primary underline" onclick={() => (removeSecret = false)}
			>{m.settings_sign_in_field_secret_keep()}</button
		>
	{:else}
		{m.settings_sign_in_field_secret_set()} ·
		<button
			type="button"
			class="text-primary underline"
			onclick={() => ((removeSecret = true), (secret = ''))}
			>{m.settings_sign_in_field_secret_remove()}</button
		>
	{/if}
{/snippet}

<!-- The provider on the left, the client Rezepte is registered as on the
     right; on a phone the two sections stack in that order. -->
<BaseDialog bind:open wide>
	<Dialog.Title class="font-display text-heading font-medium"
		>{m.settings_sign_in_dialog_title()}</Dialog.Title
	>
	<Dialog.Description class="mt-2 text-body text-text-muted">
		{m.settings_sign_in_dialog_hint()}
	</Dialog.Description>
	<form onsubmit={submit} class="mt-5 space-y-4">
		<div class="grid gap-x-6 gap-y-8 md:grid-cols-2">
			<div class="space-y-4">
				{@render section(m.settings_sign_in_section_provider())}
				<Input
					id="sign-in-issuer"
					inputmode="url"
					label={m.settings_sign_in_field_issuer()}
					autocomplete="off"
					required
					hint={m.settings_sign_in_field_issuer_hint()}
					error={issuerError}
					oninput={() => (issuerError = null)}
					bind:value={issuer}
				/>
				<Input
					id="sign-in-name"
					label={m.settings_sign_in_field_name()}
					autocomplete="off"
					placeholder={m.settings_sign_in_default_name()}
					hint={m.settings_sign_in_field_name_preview({
						name: providerName({ ...current, name: name.trim() })
					})}
					bind:value={name}
				/>
			</div>
			<div class="space-y-4">
				{@render section(m.settings_sign_in_section_client())}
				<Input
					id="sign-in-client-id"
					label={m.settings_sign_in_field_client_id()}
					autocomplete="off"
					required
					error={clientIdError}
					oninput={() => (clientIdError = null)}
					bind:value={clientId}
				/>
				<Input
					id="sign-in-secret"
					type="password"
					label={m.settings_sign_in_field_secret()}
					autocomplete="new-password"
					aside={current.secretSet ? secretState : undefined}
					hint={current.secretSet
						? m.settings_sign_in_field_secret_hint()
						: m.settings_sign_in_field_secret_hint_none()}
					error={secretError}
					oninput={() => ((secretError = null), (removeSecret = false))}
					bind:value={secret}
				/>
			</div>
		</div>
		<div class="flex justify-end gap-2.5 border-t border-border pt-4">
			<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
			<Button type="submit" disabled={saving}>{m.common_save()}</Button>
		</div>
	</form>
</BaseDialog>

<ConfirmDialog
	bind:open={switchOpen}
	title={m.settings_sign_in_switch_title()}
	text={connectedWarning(current)}
	confirmLabel={m.settings_sign_in_switch_confirm()}
	destructive
	onconfirm={() => void save()}
/>
