<script lang="ts">
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import { updateUser, type PersonEntry } from '#lib/api/users.js';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import {
		passwordErrorsFromApi,
		validateNewPassword,
		type PasswordErrors
	} from '#lib/settings/password.js';
	import NewPasswordFields from './NewPasswordFields.svelte';

	// Takes a nullable user and stays mounted: wrapping the dialog in an
	// `{#if user}` tore it down the moment the page dropped its target, so the
	// close transition never played. The page keeps the target until the next
	// open, which also keeps the title in place while the dialog animates out.
	let {
		open = $bindable(false),
		user,
		onreset
	}: {
		open?: boolean;
		user: PersonEntry | null;
		/** The password is stored; the caller shows that the account has one. */
		onreset: (user: PersonEntry) => void;
	} = $props();

	let password = $state('');
	// Typed twice, like the own password change: the admin passes this one on
	// to someone else, and a typo nobody saw would lock that person out.
	let repeat = $state('');
	let errors = $state<PasswordErrors>({});
	let saving = $state(false);

	// Fresh fields every time the dialog opens.
	$effect(() => {
		if (open) {
			password = '';
			repeat = '';
			errors = {};
		}
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (saving) return;
		if (!user) {
			return;
		}
		errors = validateNewPassword(password, repeat);
		if (Object.keys(errors).length > 0) {
			return;
		}
		saving = true;
		try {
			await updateUser(user.id, { password });
			onreset(user);
			toast.success(m.users_password_reset());
			open = false;
		} catch (failure) {
			if (failure instanceof ApiError && failure.status === 422) {
				errors = passwordErrorsFromApi(failure.errors);
				if (!errors.next) toast.error(m.users_update_error());
			} else if (!isSignedOut(failure)) {
				// 401 already redirects to the login page (see #lib/api/client).
				toast.error(m.users_update_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

<BaseDialog bind:open>
	<Dialog.Title class="font-display text-heading font-medium">
		{m.users_reset_title({ username: user?.username ?? '' })}
	</Dialog.Title>
	<Dialog.Description class="mt-2 text-body text-text-muted">
		{m.users_reset_description()}
	</Dialog.Description>
	<form onsubmit={submit} class="mt-5 space-y-4">
		<NewPasswordFields
			id="reset-password"
			bind:password
			bind:repeat
			bind:errors
			userInputs={[user?.username ?? '', user?.displayName ?? '']}
		/>
		<DialogActions class="pt-2">
			<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
			<Button type="submit" busy={saving}>{m.users_reset_submit()}</Button>
		</DialogActions>
	</form>
</BaseDialog>
