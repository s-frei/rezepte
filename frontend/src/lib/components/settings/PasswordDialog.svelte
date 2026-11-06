<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { session } from '#lib/auth.svelte.js';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import PasswordForm from './PasswordForm.svelte';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	// Read once per opening: setting a first password flips it, and the
	// title must not change under the closing dialog.
	let hadPassword = $state(true);
	$effect(() => {
		if (open) hadPassword = !!session.user?.hasPassword;
	});
</script>

<BaseDialog bind:open>
	<Dialog.Title class="font-display text-heading font-medium">
		{hadPassword ? m.settings_password_title() : m.settings_password_set_title()}
	</Dialog.Title>
	<Dialog.Description class="sr-only">
		{hadPassword ? m.settings_password_title() : m.settings_password_set_hint()}
	</Dialog.Description>
	<div class="mt-5">
		<PasswordForm ondone={() => (open = false)} oncancel={() => (open = false)} />
	</div>
</BaseDialog>
