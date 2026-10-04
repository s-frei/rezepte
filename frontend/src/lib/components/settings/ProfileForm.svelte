<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import MailQuestionMark from '@lucide/svelte/icons/mail-question-mark';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { getPasswordReset, me, resendConfirmation, updateOwnProfile } from '$lib/api/auth';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import { session } from '$lib/auth.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { m } from '$lib/paraglide/messages';

	// Seeded once, not derived: the root layout's `load` puts the session in
	// place before any page renders, and from then on the field belongs to
	// whoever is typing in it.
	let displayName = $state(session.user?.displayName ?? '');
	let email = $state(session.user?.email ?? '');
	let nameError = $state<string | null>(null);
	let emailError = $state<string | null>(null);
	let saving = $state(false);

	// The confirmed line only applies to the address still on the account -
	// the moment somebody edits it, the confirmation no longer says anything
	// about what is now typed.
	const emailConfirmed = $derived(
		!!session.user?.emailVerified && email.trim() === (session.user?.email ?? '')
	);

	// Whether mail is on: without it there is nothing to send again.
	let mailOn = $state(false);
	// Set when the button's last send failed.
	let sendFailed = $state(false);
	let resending = $state(false);

	onMount(() => {
		getPasswordReset()
			.then((info) => (mailOn = info.available))
			.catch(() => {});
	});

	const savedEmail = $derived(session.user?.email ?? '');
	// Only for the address still on the account, like the confirmed line.
	const emailUnconfirmed = $derived(
		!!savedEmail && !session.user?.emailVerified && email.trim() === savedEmail
	);
	// From the service: a link mailed to this address is still open. Only
	// then does the line say a mail went out.
	const pending = $derived(!!session.user?.emailConfirmationPending);

	async function resend() {
		resending = true;
		try {
			await resendConfirmation();
			sendFailed = false;
			if (session.user) session.user = { ...session.user, emailConfirmationPending: true };
			toast.success(m.settings_profile_email_resent({ address: savedEmail }));
		} catch (failure) {
			if (failure instanceof ApiError && failure.status === 429) {
				toast.error(m.settings_profile_email_throttled());
			} else if (failure instanceof ApiError && failure.status === 409) {
				// Changed elsewhere: confirmed in another tab, or mail turned
				// off. Nothing failed here, so the page catches up instead.
				sendFailed = false;
				const [user, info] = await Promise.all([
					me().catch(() => session.user),
					getPasswordReset().catch(() => ({ available: mailOn }))
				]);
				session.user = user;
				mailOn = info.available;
			} else if (failure instanceof ApiError && failure.status === 502) {
				// The service drops a link whose mail failed.
				sendFailed = true;
				if (session.user) session.user = { ...session.user, emailConfirmationPending: false };
			} else if (!isSignedOut(failure)) {
				// Unknown how far it got, so the line stays as it was.
				toast.error(m.settings_profile_email_resend_error());
			}
		} finally {
			resending = false;
		}
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		nameError = null;
		emailError = null;
		saving = true;
		try {
			// An empty name is not an error: the API falls back to the login
			// name, which is how a person takes a display name back off. An
			// empty email clears it the same way.
			const user = await updateOwnProfile({
				displayName: displayName.trim(),
				email: email.trim()
			});
			// Straight into the shared session, so the top bar, the recipe
			// cards and this page's own avatar all change at once.
			// A new address starts over. No link open after a save can mean a
			// failed mail or one the minute's limit held back, so the line
			// stays neutral and offers the button either way.
			if (user.email !== savedEmail) sendFailed = false;
			session.user = user;
			displayName = user.displayName;
			email = user.email;
			toast.success(m.settings_profile_saved());
		} catch (failure) {
			// The name has two failure modes: too long, or a control character
			// (a pasted newline reaches this one; the length one is mostly
			// caught client-side by `maxlength`) - both located at
			// `body.displayName`, so the message is what tells them apart. The
			// email has one, located at `body.email`.
			if (failure instanceof ApiError && failure.status === 422) {
				const nameIssue = failure.errors.find((e) => e.location === 'body.displayName');
				if (nameIssue) {
					nameError = nameIssue.message.includes('control character')
						? m.settings_profile_name_control_char()
						: m.settings_profile_name_invalid();
				}
				const emailIssue = failure.errors.some((e) => e.location === 'body.email');
				if (emailIssue) {
					emailError = m.settings_profile_email_invalid();
				}
				if (!nameIssue && !emailIssue) {
					toast.error(m.settings_profile_error());
				}
			} else if (!isSignedOut(failure)) {
				// 401 already redirects to the login page (see $lib/api/client).
				toast.error(m.settings_profile_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

<form onsubmit={submit} class="space-y-4">
	<!-- `maxlength` mirrors the API's 64-rune limit, so the usual way to
	     overrun it - pasting - is caught before the request. `counter` is the
	     same number again because it counts towards that same limit. -->
	<Input
		id="display-name"
		label={m.settings_profile_name_label()}
		autocomplete="nickname"
		maxlength={64}
		counter={64}
		bind:value={displayName}
		oninput={() => (nameError = null)}
		error={nameError}
	/>
	<div class="space-y-1.5">
		<Input
			id="email"
			label={m.settings_profile_email_label()}
			type="email"
			autocomplete="email"
			maxlength={254}
			bind:value={email}
			oninput={() => (emailError = null)}
			error={emailError}
			hint={emailConfirmed || emailUnconfirmed ? undefined : m.settings_profile_email_hint()}
		/>
		{#if emailConfirmed}
			<p class="flex items-center gap-1 text-micro text-text-muted">
				<Check class="size-3.5" aria-hidden="true" />
				{m.settings_profile_email_verified()}
			</p>
		{:else if emailUnconfirmed}
			<!-- One run of text beside the symbol, so a wrapped line keeps its
			     indent and the button follows the last word. -->
			<p class="flex items-start gap-1 text-micro text-text-muted">
				<MailQuestionMark class="mt-px size-3.5 shrink-0" aria-hidden="true" />
				<span class="min-w-0">
					{#if !mailOn}
						{m.settings_profile_email_unconfirmed()}
					{:else if sendFailed}
						<span class="text-destructive">{m.settings_profile_email_send_failed()}</span>
					{:else if pending}
						{m.settings_profile_email_pending({ address: savedEmail })}
					{:else}
						{m.settings_profile_email_unconfirmed()}
					{/if}
					{#if mailOn}
						<button
							type="button"
							disabled={resending}
							onclick={resend}
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
	<!-- Beside the field's right edge at every width, because the password
	     form in the next card down ends exactly there: two settings cards
	     stacked on one page whose buttons are shaped differently read as an
	     oversight, not as a decision about thumbs. -->
	<div class="flex justify-end">
		<Button type="submit" disabled={saving}>{m.settings_profile_save()}</Button>
	</div>
</form>
