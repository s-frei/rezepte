<script lang="ts">
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import Check from '@lucide/svelte/icons/check';
	import MailQuestionMark from '@lucide/svelte/icons/mail-question-mark';
	import { Dialog } from 'bits-ui';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { getPasswordReset } from '#lib/api/auth.js';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let {
		open = $bindable(false),
		email,
		emailVerified,
		pending = false,
		save,
		resend,
		onerror
	}: {
		open?: boolean;
		/** The address on the account; empty when none. */
		email: string;
		emailVerified: boolean;
		/** A confirmation mail to `email` is still open (own account only). */
		pending?: boolean;
		/** Stores the trimmed address; the caller takes the saved account from it. */
		save: (email: string) => Promise<unknown>;
		/**
		 * Sends the confirmation mail again - the own account's alone, so the
		 * person page leaves it out. A 409 means it changed elsewhere; the
		 * caller refreshes what it shows.
		 */
		resend?: () => Promise<void>;
		/** Toasts a failure the field does not show; by default "could not be saved". */
		onerror?: (error: unknown) => void;
	} = $props();

	const uid = $props.id();

	let address = $state('');
	let error = $state<string | null>(null);
	let saving = $state(false);
	// Whether mail is on: without it there is nothing to send, and a new
	// address stays unconfirmed.
	let mailOn = $state(false);
	// Set when the button's last send failed.
	let sendFailed = $state(false);
	let resending = $state(false);

	const typed = $derived(address.trim());
	const changed = $derived(typed !== email);
	// The state line only speaks about the address still on the account.
	const confirmed = $derived(!!email && emailVerified && !changed);
	const unconfirmed = $derived(!!email && !emailVerified && !changed);

	// A fresh form every time the dialog opens. Only `open` is tracked, so a
	// session refresh while typing does not wipe the field.
	$effect(() => {
		if (open) {
			untrack(() => {
				address = email;
				error = null;
				saving = false;
			});
			getPasswordReset()
				.then((info) => (mailOn = info.available))
				.catch(() => {});
		}
	});

	async function sendAgain() {
		if (!resend) return;
		resending = true;
		try {
			await resend();
			sendFailed = false;
			toast.success(m.settings_profile_email_resent({ address: email }));
		} catch (failure) {
			if (failure instanceof ApiError && failure.status === 429) {
				toast.error(m.settings_profile_email_throttled());
			} else if (failure instanceof ApiError && failure.status === 409) {
				// Confirmed in another tab, or mail turned off: nothing failed here.
				sendFailed = false;
				mailOn = await getPasswordReset()
					.then((info) => info.available)
					.catch(() => mailOn);
			} else if (failure instanceof ApiError && failure.status === 502) {
				// The service drops a link whose mail failed.
				sendFailed = true;
			} else if (!isSignedOut(failure)) {
				toast.error(m.settings_profile_email_resend_error());
			}
		} finally {
			resending = false;
		}
	}

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
			// An empty address clears it.
			await save(typed);
			sendFailed = false;
			toast.success(m.profile_email_saved());
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
				hint={changed && typed
					? mailOn
						? m.users_edit_email_mail_hint()
						: m.users_edit_email_no_mail_hint()
					: undefined}
			/>
			{#if confirmed}
				<p class="flex items-center gap-1 text-micro text-text-muted">
					<Check class="size-3.5" aria-hidden="true" />
					{m.settings_profile_email_verified()}
				</p>
			{:else if unconfirmed}
				<!-- One run of text beside the symbol, so a wrapped line keeps its
				     indent and the button follows the last word. -->
				<p class="flex items-start gap-1 text-micro text-text-muted">
					<MailQuestionMark class="mt-px size-3.5 shrink-0" aria-hidden="true" />
					<span class="min-w-0">
						{#if resend && mailOn && sendFailed}
							<span class="text-destructive">{m.settings_profile_email_send_failed()}</span>
						{:else if resend && mailOn && pending}
							{m.settings_profile_email_pending({ address: email })}
						{:else}
							{m.settings_profile_email_unconfirmed()}
						{/if}
						{#if resend && mailOn}
							<button
								type="button"
								disabled={resending}
								onclick={sendAgain}
								class="font-semibold whitespace-nowrap text-primary underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-50"
							>
								{pending || sendFailed
									? m.settings_profile_email_resend()
									: m.settings_profile_email_send()}
							</button>
						{/if}
					</span>
				</p>
			{/if}
		</div>
		<DialogActions class="pt-2">
			<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
			<Button type="submit" busy={saving}>{m.common_save()}</Button>
		</DialogActions>
	</form>
</BaseDialog>
