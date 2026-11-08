<script lang="ts">
	import Mail from '@lucide/svelte/icons/mail';
	import type { Snippet } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { MailWindow } from '#lib/api/auth.js';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import SettingsRow from '#lib/components/settings/SettingsRow.svelte';
	import LifetimeBar from '#lib/components/ui/LifetimeBar.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { formatDate } from '#lib/recipe/format.js';
	import { emailAction, emailState } from '#lib/user/email-state.js';

	let {
		email,
		emailVerified,
		emailConfirmation,
		emailSetupLink,
		mailOn,
		resend,
		onchanged,
		none,
		control
	}: {
		/** The address on the account; empty when none. */
		email: string;
		emailVerified: boolean;
		emailConfirmation?: MailWindow;
		emailSetupLink?: MailWindow;
		/** Whether Rezepte sends mail: without it there is nothing to send. Null until known. */
		mailOn: boolean | null;
		/** Mails a new confirmation link; the row toasts how it went. */
		resend: () => Promise<void>;
		/** The account changed: reread it so the bar restarts or the row catches up. */
		onchanged: () => void;
		/** What the row says without an address. */
		none: string;
		/** The row's one control, "Change". */
		control: Snippet;
	} = $props();

	const status = $derived(
		emailState({ email, emailVerified, emailConfirmation, emailSetupLink }, mailOn)
	);
	const at = $derived(email.indexOf('@'));
	// Without an address, confirmed, or before it is known whether mail is on
	// for an address nothing went to: no card.
	const hasCard = $derived(
		status.kind !== 'none' &&
			status.kind !== 'confirmed' &&
			!(status.kind === 'unsent' && status.mailOn === null)
	);
	let sending = $state(false);

	async function send() {
		if (sending) return;
		sending = true;
		try {
			await resend();
			toast.success(m.settings_profile_email_resent({ address: email }));
			onchanged();
		} catch (failure) {
			if (failure instanceof ApiError && failure.status === 429) {
				toast.error(m.settings_profile_email_throttled());
			} else if (failure instanceof ApiError && failure.status === 409) {
				// Confirmed meanwhile, or mail turned off: nothing failed here.
				onchanged();
			} else if (failure instanceof ApiError && failure.status === 502) {
				toast.error(m.settings_profile_email_send_failed());
				onchanged();
			} else if (!isSignedOut(failure)) {
				toast.error(m.settings_profile_email_resend_error());
			}
		} finally {
			sending = false;
		}
	}
</script>

{#snippet badge()}
	<span
		class="ml-1.5 inline-flex items-center rounded-pill px-2 align-[1px] text-micro font-semibold whitespace-nowrap {status.kind ===
		'confirmed'
			? 'bg-success-soft text-success-foreground'
			: 'bg-accent text-accent-foreground'}"
	>
		{status.kind === 'confirmed'
			? m.email_badge_confirmed()
			: status.kind === 'expired'
				? m.email_badge_expired()
				: m.email_badge_unconfirmed()}
	</span>
{/snippet}
{#snippet card()}
	{@const kind = emailAction(status, mailOn)}
	{@const action =
		kind === 'resend'
			? m.email_action_resend()
			: kind === 'new-link'
				? m.email_action_new_link()
				: kind === 'send'
					? m.email_action_send()
					: undefined}
	<div class="rounded-md bg-background p-3">
		<div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-caption">
			<span>
				{status.kind === 'pending'
					? m.email_card_sent()
					: status.kind === 'expired'
						? m.email_card_not_opened()
						: status.kind === 'setup-link'
							? m.email_card_setup_link()
							: status.kind === 'unsent' && status.mailOn
								? m.email_card_unsent()
								: m.email_card_mail_off()}
			</span>
			{#if action}
				<button
					type="button"
					disabled={sending}
					onclick={send}
					class="ml-auto font-semibold whitespace-nowrap text-primary underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-50"
				>
					{action}
				</button>
			{/if}
		</div>
		{#if 'window' in status}
			<div class="mt-2">
				<LifetimeBar
					compact
					item={{ createdAt: status.window.sentAt, expiresAt: status.window.expiresAt }}
				/>
			</div>
			<p class="sr-only">
				{(status.kind === 'expired' ? m.email_card_dates_expired : m.email_card_dates)({
					sent: formatDate(status.window.sentAt),
					expires: formatDate(status.window.expiresAt)
				})}
			</p>
		{/if}
	</div>
{/snippet}

<!-- What happened to the address, with it: a badge on the label's line and,
     while it is unconfirmed, a card under it saying which mail went out, until
     when its link works, and the way to send it again. The badge has no icon or dot
     so it stays beside "Email" at 320px next to the control. -->
<SettingsRow
	icon={Mail}
	label={m.users_field_email()}
	{control}
	badge={status.kind === 'none' ? undefined : badge}
	below={hasCard ? card : undefined}
>
	{#snippet value()}
		{#if at > 0}
			<!-- Breaks after the @ when it has to, never inside a word. -->
			{email.slice(0, at + 1)}<wbr />{email.slice(at + 1)}
		{:else if email}
			{email}
		{:else}
			{none}
		{/if}
	{/snippet}
</SettingsRow>
