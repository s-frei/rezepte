<script lang="ts">
	import { Dialog, RadioGroup } from 'bits-ui';
	import { ApiError } from '$lib/api/client';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { m } from '$lib/paraglide/messages';

	// The row action's step before a setup link goes out, shown only while
	// mail is on: by mail to an address, or the link alone to pass on. The
	// same choice as Add account's sign-in mode, so the two read alike.
	let {
		open = $bindable(false),
		displayName,
		email,
		onsend,
		onshowonly
	}: {
		open?: boolean;
		displayName: string;
		email: string;
		onsend: (address: string) => Promise<void>;
		onshowonly: () => Promise<void>;
	} = $props();

	let mode = $state<'mail' | 'self'>('mail');
	let address = $state('');
	let busy = $state(false);
	let error = $state<string | null>(null);

	$effect(() => {
		if (open) {
			mode = 'mail';
			address = email;
			error = null;
		}
	});

	async function run(action: () => Promise<void>) {
		busy = true;
		error = null;
		try {
			await action();
			open = false;
		} catch (e) {
			// The caller toasts everything but a refused address, which
			// belongs under the field it came from.
			if (
				e instanceof ApiError &&
				e.status === 422 &&
				e.errors.some((x) => x.location === 'body.email')
			) {
				error = m.settings_profile_email_invalid();
			} else {
				open = false;
			}
		} finally {
			busy = false;
		}
	}

	function submit(event: SubmitEvent) {
		event.preventDefault();
		void run(mode === 'mail' ? () => onsend(address.trim()) : onshowonly);
	}
</script>

<BaseDialog bind:open>
	<Dialog.Title class="font-display text-heading font-medium">
		{m.setup_link_dialog_title({ name: displayName })}
	</Dialog.Title>
	<Dialog.Description class="mt-2 text-body text-text-muted">
		{m.setup_link_send_hint()}
	</Dialog.Description>
	<form class="mt-5" onsubmit={submit}>
		<!-- Add account's sign-in group, with the address inside the mail
		     option while it is picked: a field cannot sit in the radio button
		     itself, so it follows it on the same tint. -->
		<RadioGroup.Root
			bind:value={mode}
			aria-label={m.setup_link_choice_label()}
			class="grid grid-cols-1 overflow-hidden rounded-md border border-border"
		>
			{@render option('mail', m.setup_link_choice_mail())}
			{#if mode === 'mail'}
				<div class="bg-accent px-4 pb-4">
					<Input
						id="send-link-email"
						type="email"
						required
						autocomplete="off"
						maxlength={254}
						label={m.users_field_email()}
						hint={m.setup_link_send_email_hint({ name: displayName })}
						bind:value={address}
						oninput={() => (error = null)}
						{error}
					/>
				</div>
			{/if}
			{@render option('self', m.setup_link_choice_self())}
		</RadioGroup.Root>
		<div class="mt-6 flex justify-end gap-2.5 border-t border-border pt-4">
			<Button variant="ghost" disabled={busy} onclick={() => (open = false)}>
				{m.common_cancel()}
			</Button>
			<Button type="submit" disabled={busy || (mode === 'mail' && !address.trim())}>
				{mode === 'mail' ? m.setup_link_send() : m.setup_link_show()}
			</Button>
		</div>
	</form>
</BaseDialog>

{#snippet option(value: 'mail' | 'self', label: string)}
	<RadioGroup.Item
		{value}
		class="flex items-center gap-2 border-border bg-surface-elevated px-4 py-3 text-left text-body-sm font-semibold transition not-first:border-t data-[state=checked]:bg-accent"
	>
		{#snippet children({ checked })}
			<span
				aria-hidden="true"
				class="size-4 shrink-0 rounded-full border {checked
					? 'border-[5px] border-primary bg-surface'
					: 'border-border bg-surface-elevated'}"
			></span>
			{label}
		{/snippet}
	</RadioGroup.Item>
{/snippet}
