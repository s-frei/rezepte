<script lang="ts" module>
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import { m } from '#lib/paraglide/messages.js';

	export type TestMailResult = { ok: true; to: string } | { ok: false; message: string };

	/**
	 * What the owner reads under the row when a test or a save fails: the
	 * mail server's own reply for a refused delivery (502), since that is
	 * what there is to fix, and catalog copy for everything else - the
	 * API's own words are English.
	 */
	export function mailProblem(error: unknown, fallback: string): string {
		if (!(error instanceof ApiError)) return fallback;
		if (error.status === 502) return error.message;
		if (error.status !== 422) return fallback;
		const at = error.errors.map((e) => e.location);
		if (at.includes('body.to')) return m.settings_profile_email_invalid();
		if (at.some((l) => l.endsWith('.port'))) return m.settings_mail_field_port_invalid();
		return m.settings_mail_config_invalid();
	}
</script>

<script lang="ts">
	import CircleCheckIcon from '#lib/components/icons/CircleCheckIcon.svelte';
	import SendIcon from '#lib/components/icons/SendIcon.svelte';
	import { sendTestMail, type MailConfigInput } from '#lib/api/mail.js';
	import { session } from '#lib/auth.svelte.js';
	import Button from '#lib/components/ui/Button.svelte';
	import Input from '#lib/components/ui/Input.svelte';

	let {
		config,
		result = $bindable(null)
	}: {
		/** Unsaved values to test; omitted tests the configuration in force. Null means the form shows what is wrong, so nothing is sent. */
		config?: () => MailConfigInput | null;
		/** The last outcome, shown under the row; the dialog also puts a refused save here. */
		result?: TestMailResult | null;
	} = $props();

	let to = $state(session.user?.email ?? '');
	let sending = $state(false);
	let sendIcon = $state<SendIcon>();
	const ready = $derived(!sending && to.trim() !== '');

	async function send() {
		if (!ready) return;
		const override = config?.();
		if (override === null) return;
		sending = true;
		sendIcon?.play();
		try {
			await sendTestMail(to.trim(), override);
			result = { ok: true, to: to.trim() };
		} catch (error) {
			if (isSignedOut(error)) return;
			result = { ok: false, message: mailProblem(error, m.settings_mail_test_error()) };
		} finally {
			sending = false;
		}
	}

	// Not a <form>: in the dialog the row sits inside the settings form, and
	// Enter here must send the test, not save.
	function onkeydown(event: KeyboardEvent) {
		if (event.key !== 'Enter') return;
		event.preventDefault();
		void send();
	}
</script>

<div>
	<div class="flex flex-wrap items-end gap-2.5">
		<div class="min-w-0 flex-1 basis-56">
			<Input
				id={config ? 'mail-test-to-unsaved' : 'mail-test-to'}
				type="email"
				label={m.settings_mail_test_label()}
				autocomplete="email"
				bind:value={to}
				{onkeydown}
			/>
		</div>
		<Button
			variant="secondary"
			class="h-11"
			disabled={to.trim() === ''}
			busy={sending}
			onclick={() => void send()}
		>
			<SendIcon bind:this={sendIcon} class="size-4" />
			{config ? m.settings_mail_test_send_unsaved() : m.settings_mail_test_send()}
		</Button>
	</div>
	{#if result}
		<p
			role={result.ok ? 'status' : 'alert'}
			class="mt-3 flex items-start gap-2 rounded-md px-3.5 py-2.5 text-body-sm font-semibold break-words {result.ok
				? 'bg-success-soft text-success-foreground'
				: 'bg-destructive-soft text-destructive'}"
		>
			{#if result.ok}
				<!-- Keyed, so a second test that succeeds draws its check again. -->
				{#key result}
					<CircleCheckIcon class="mt-0.5 size-4 shrink-0" />
				{/key}
				{config
					? m.settings_mail_test_sent_unsaved({ to: result.to })
					: m.settings_mail_test_sent({ to: result.to })}
			{:else}
				{result.message}
			{/if}
		</p>
	{/if}
</div>
