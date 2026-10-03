<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import {
		saveMailSettings,
		type MailConfigInput,
		type MailSecurity,
		type MailSettings
	} from '$lib/api/mail';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import SegmentedControl from '$lib/components/ui/SegmentedControl.svelte';
	import { m } from '$lib/paraglide/messages';
	import TestMailRow, { type TestMailResult } from './TestMailRow.svelte';

	let {
		open = $bindable(false),
		current,
		onsaved
	}: { open?: boolean; current: MailSettings; onsaved: (next: MailSettings) => void } = $props();

	let from = $state('');
	let fromName = $state('Rezepte');
	let username = $state('');
	let password = $state('');
	let host = $state('');
	let port = $state('587');
	let security = $state<MailSecurity>('starttls');
	let result = $state<TestMailResult | null>(null);
	let saving = $state(false);
	let portError = $state<string | null>(null);

	const securityOptions = $derived<{ value: MailSecurity; label: string }[]>([
		{ value: 'starttls', label: 'STARTTLS' },
		{ value: 'tls', label: 'TLS' },
		{ value: 'none', label: m.settings_mail_security_none() }
	]);

	// Refilled from what is stored every time the dialog opens, so a cancelled
	// edit never lingers into the next one.
	$effect(() => {
		if (!open) return;
		from = current.from;
		fromName = current.fromName;
		username = current.username;
		password = '';
		host = current.host;
		port = String(current.port);
		security = current.security;
		result = null;
		portError = null;
	});

	function input(): MailConfigInput {
		return {
			host: host.trim(),
			port: Number(port),
			security,
			username: username.trim(),
			from: from.trim(),
			fromName: fromName.trim(),
			// Untouched means "keep": the field is never filled with the stored password.
			...(password || !current.passwordSet ? { password } : {})
		};
	}

	// Checked here, so a typo gets its own field error instead of the API's
	// generic "validation failed".
	function portOk(): boolean {
		const n = Number(port.trim());
		const ok = /^\d+$/.test(port.trim()) && n >= 1 && n <= 65535;
		portError = ok ? null : m.settings_mail_field_port_invalid();
		return ok;
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		if (!portOk()) return;
		saving = true;
		try {
			onsaved(await saveMailSettings(input()));
			toast.success(m.settings_mail_saved());
			open = false;
		} catch (error) {
			if (isSignedOut(error)) return;
			// A refused configuration says why ("credentials need starttls or
			// tls"); it goes where the test result goes, under the test row.
			if (error instanceof ApiError && error.status === 422) {
				result = { ok: false, message: error.message };
			} else {
				toast.error(m.settings_mail_save_error());
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

<!-- Sender and sign-in on the left, the server on the right; on a phone the
     three sections stack in that order. -->
<BaseDialog bind:open wide>
	<Dialog.Title class="font-display text-heading font-medium"
		>{m.settings_mail_dialog_title()}</Dialog.Title
	>
	<Dialog.Description class="mt-2 text-body text-text-muted">
		{m.settings_mail_dialog_hint()}
	</Dialog.Description>
	<form onsubmit={save} class="mt-5 space-y-4">
		<div class="grid gap-x-6 gap-y-8 md:grid-cols-2">
			<div class="space-y-4">
				{@render section(m.settings_mail_section_sender())}
				<Input
					id="mail-from"
					type="email"
					label={m.settings_mail_field_from()}
					autocomplete="off"
					required
					bind:value={from}
				/>
				<Input
					id="mail-from-name"
					label={m.settings_mail_field_from_name()}
					autocomplete="off"
					hint={from.trim()
						? m.settings_mail_field_from_name_preview({
								name: fromName.trim() || 'Rezepte',
								address: from.trim()
							})
						: undefined}
					bind:value={fromName}
				/>
				<div class="pt-4">{@render section(m.settings_mail_section_signin())}</div>
				<Input
					id="mail-username"
					label={m.settings_mail_field_username()}
					autocomplete="off"
					bind:value={username}
				/>
				<Input
					id="mail-password"
					type="password"
					label={m.settings_mail_field_password()}
					autocomplete="new-password"
					placeholder={current.passwordSet ? m.settings_mail_field_password_set() : undefined}
					hint={current.passwordSet ? m.settings_mail_field_password_keep() : undefined}
					bind:value={password}
				/>
			</div>
			<div class="space-y-4">
				{@render section(m.settings_mail_section_server())}
				<div class="grid grid-cols-[2fr_1fr] gap-3">
					<Input
						id="mail-host"
						label={m.settings_mail_field_host()}
						autocomplete="off"
						required
						bind:value={host}
					/>
					<Input
						id="mail-port"
						label={m.settings_mail_field_port()}
						inputmode="numeric"
						autocomplete="off"
						required
						bind:value={port}
						oninput={() => (portError = null)}
						error={portError}
					/>
				</div>
				<div class="space-y-1.5">
					<span class="block text-caption font-semibold">
						{m.settings_mail_field_security()}
					</span>
					<SegmentedControl
						bind:value={security}
						options={securityOptions}
						label={m.settings_mail_field_security()}
					/>
					<p class="text-micro text-text-muted">{m.settings_mail_field_security_hint()}</p>
				</div>
			</div>
		</div>
		<div class="pt-2"><TestMailRow config={() => (portOk() ? input() : null)} bind:result /></div>
		<div class="flex justify-end gap-2.5 border-t border-border pt-4">
			<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
			<Button type="submit" disabled={saving}>{m.common_save()}</Button>
		</div>
	</form>
</BaseDialog>
