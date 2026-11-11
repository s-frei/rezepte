<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { changePassword } from '#lib/api/auth.js';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import { session } from '#lib/auth.svelte.js';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import { withoutErrors } from '#lib/form-errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import {
		passwordErrorsFromApi,
		validateNewPassword,
		type PasswordErrors
	} from '#lib/settings/password.js';
	import NewPasswordFields from './NewPasswordFields.svelte';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	// Read once per opening: setting a first password flips it, and the
	// title must not change under the closing dialog.
	let hadPassword = $state(true);
	$effect(() => {
		if (open) hadPassword = !!session.user?.hasPassword;
	});

	let current = $state('');
	let next = $state('');
	let repeat = $state('');
	let errors = $state<PasswordErrors>({});
	let saving = $state(false);

	// An account with no password yet has nothing to confirm - the session
	// itself is the proof, same as the profile write.
	const hasPassword = $derived(!!session.user?.hasPassword);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (saving) return;
		errors = validateNewPassword(next, repeat);
		if (hasPassword && current === '') {
			errors = { ...errors, current: m.settings_password_current_required() };
		}
		if (Object.keys(errors).length > 0) {
			return;
		}
		saving = true;
		try {
			await changePassword(hasPassword ? current : undefined, next);
			if (session.user) {
				session.user = { ...session.user, hasPassword: true };
			}
			toast.success(m.settings_password_changed());
			current = '';
			next = '';
			repeat = '';
			open = false;
		} catch (error) {
			if (error instanceof ApiError && error.status === 422) {
				errors = passwordErrorsFromApi(error.errors);
				if (Object.keys(errors).length === 0) {
					toast.error(m.settings_password_error());
				}
			} else if (!isSignedOut(error)) {
				// 401 already redirects to the login page (see #lib/api/client).
				toast.error(m.settings_password_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

<BaseDialog bind:open>
	<Dialog.Title class="font-display text-heading font-medium">
		{hadPassword ? m.settings_password_title() : m.settings_password_set_title()}
	</Dialog.Title>
	<Dialog.Description class="sr-only">
		{hadPassword ? m.settings_password_title() : m.settings_password_set_hint()}
	</Dialog.Description>
	<div class="mt-5">
		<form onsubmit={submit} class="space-y-4">
			<!-- Tells a password manager which account the new password belongs to;
			     without it the change is saved as a second, nameless entry. -->
			<input
				type="text"
				name="username"
				autocomplete="username"
				value={session.user?.username ?? ''}
				readonly
				hidden
			/>
			{#if hasPassword}
				<Input
					id="current-password"
					label={m.settings_password_current()}
					type="password"
					autocomplete="current-password"
					bind:value={current}
					oninput={() => (errors = withoutErrors(errors, ['current']))}
					error={errors.current ?? null}
				/>
			{:else}
				<p class="text-caption text-text-muted">{m.settings_password_set_hint()}</p>
			{/if}
			<NewPasswordFields
				id="new-password"
				bind:password={next}
				bind:repeat
				bind:errors
				userInputs={[session.user?.username ?? '', session.user?.displayName ?? '']}
			/>
			<!-- The dialog's title already names the password, and a longer label
			     would wrap beside the cancel button on a phone. -->
			<DialogActions class="pt-2">
				<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
				<Button type="submit" busy={saving}>{m.common_save()}</Button>
			</DialogActions>
		</form>
	</div>
</BaseDialog>
