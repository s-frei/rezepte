<script lang="ts" module>
	export type TestMailResult = { ok: true; to: string } | { ok: false; message: string };
</script>

<script lang="ts">
	import CircleCheckIcon from '$lib/components/icons/CircleCheckIcon.svelte';
	import SendIcon from '$lib/components/icons/SendIcon.svelte';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import { sendTestMail, type MailConfigInput } from '$lib/api/mail';
	import { session } from '$lib/auth.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { m } from '$lib/paraglide/messages';

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
			// The SMTP server's own reply (502) or the configuration problem
			// (422), as the API words it: that is what the owner needs to fix.
			result = { ok: false, message: error instanceof ApiError ? error.message : String(error) };
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
		<Button variant="secondary" class="h-11" disabled={!ready} onclick={() => void send()}>
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
