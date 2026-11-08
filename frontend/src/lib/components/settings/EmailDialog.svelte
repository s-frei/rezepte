<script lang="ts">
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import { Dialog } from 'bits-ui';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { MailWindow } from '#lib/api/auth.js';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import MailNotice from '#lib/components/settings/MailNotice.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { mailNotice, mailWentOut } from '#lib/user/mail-notice.js';

	let {
		open = $bindable(false),
		email,
		sentAt,
		name,
		mailOn,
		save,
		onerror
	}: {
		open?: boolean;
		/** The address on the account; empty when none. */
		email: string;
		/** When the account's confirmation went out, if one did: tells a new send apart. */
		sentAt?: string;
		/** The person's first name on someone else's account; absent on one's own. */
		name?: string;
		/** Whether Rezepte sends mail, as the page read it; null until known, so no note says the wrong thing. */
		mailOn: boolean | null;
		/** Stores the trimmed address and answers the saved account. */
		save: (email: string) => Promise<{ emailConfirmation?: MailWindow }>;
		/** Toasts a failure the field does not show; by default "could not be saved". */
		onerror?: (error: unknown) => void;
	} = $props();

	const uid = $props.id();

	let address = $state('');
	let error = $state<string | null>(null);
	let saving = $state(false);

	const typed = $derived(address.trim());
	const changed = $derived(typed !== email);
	const notice = $derived(mailOn === null ? null : mailNotice(email, address, mailOn));

	// A fresh form every time the dialog opens. Only `open` is tracked, so a
	// session refresh while typing does not wipe the field.
	$effect(() => {
		if (open) {
			untrack(() => {
				address = email;
				error = null;
				saving = false;
			});
		}
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (saving) return;
		if (!changed) {
			open = false;
			return;
		}
		error = null;
		saving = true;
		try {
			// Read before saving: the caller updates the account, and with it
			// this prop, before the save resolves.
			const before = sentAt;
			// An empty address clears it.
			const saved = await save(typed);
			toast.success(
				mailWentOut(before, saved.emailConfirmation)
					? m.profile_email_saved_sent({ address: typed })
					: m.profile_email_saved()
			);
			open = false;
		} catch (failure) {
			if (
				failure instanceof ApiError &&
				failure.status === 422 &&
				failure.errors.some((e) => e.location === 'body.email')
			) {
				error = m.settings_profile_email_invalid();
			} else if (onerror) {
				onerror(failure);
			} else if (!isSignedOut(failure)) {
				// 401 already redirects to the login page (see #lib/api/client).
				toast.error(m.settings_profile_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

<BaseDialog bind:open>
	<Dialog.Title class="font-display text-heading font-medium"
		>{m.profile_email_title()}</Dialog.Title
	>
	<Dialog.Description class="mt-2 text-body text-text-muted">
		{m.settings_profile_email_hint()}
	</Dialog.Description>
	<form onsubmit={submit} class="mt-5 space-y-4">
		<div class="space-y-1.5">
			<Input
				id="{uid}-email"
				label={m.settings_profile_email_label()}
				type="email"
				autocomplete="email"
				maxlength={254}
				bind:value={address}
				oninput={() => (error = null)}
				{error}
			/>
		</div>
		{#if notice === 'send'}
			<MailNotice
				id="{uid}-notice"
				text={name
					? m.email_notice_save({ address: '\0', name })
					: m.email_notice_save_own({ address: '\0' })}
				address={typed}
			/>
		{:else if notice === 'mail-off'}
			<MailNotice id="{uid}-notice" text={m.email_notice_mail_off()} />
		{/if}
		<DialogActions class="pt-2">
			<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
			<Button type="submit" busy={saving} aria-describedby={notice ? `${uid}-notice` : undefined}>
				{notice === 'send' ? m.email_save_and_send() : m.common_save()}
			</Button>
		</DialogActions>
	</form>
</BaseDialog>
