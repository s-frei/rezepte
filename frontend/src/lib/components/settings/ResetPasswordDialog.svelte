<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import { updateUser, type UserAccount } from '$lib/api/users';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { withoutErrors } from '$lib/form-errors';
	import { m } from '$lib/paraglide/messages';
	import {
		passwordErrorsFromApi,
		validateNewPassword,
		type PasswordErrors
	} from '$lib/settings/password';
	import PasswordStrength from './PasswordStrength.svelte';

	// Takes a nullable user and stays mounted: wrapping the dialog in an
	// `{#if user}` tore it down the moment the page dropped its target, so the
	// close transition never played. The page keeps the target until the next
	// open, which also keeps the title in place while the dialog animates out.
	let { open = $bindable(false), user }: { open?: boolean; user: UserAccount | null } = $props();

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
			toast.success(m.users_password_reset());
			open = false;
		} catch (failure) {
			if (failure instanceof ApiError && failure.status === 422) {
				errors = passwordErrorsFromApi(failure.errors);
				if (!errors.next) toast.error(failure.detail ?? m.users_update_error());
			} else if (!isSignedOut(failure)) {
				// 401 already redirects to the login page (see $lib/api/client).
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
		<div class="space-y-1.5">
			<Input
				id="reset-password"
				label={m.settings_password_new()}
				type="password"
				autocomplete="new-password"
				bind:value={password}
				oninput={() => (errors = withoutErrors(errors, ['next']))}
				error={errors.next ?? null}
			/>
			<PasswordStrength {password} userInputs={[user?.username ?? '', user?.displayName ?? '']} />
		</div>
		<Input
			id="reset-password-repeat"
			label={m.settings_password_repeat()}
			type="password"
			autocomplete="new-password"
			bind:value={repeat}
			oninput={() => (errors = withoutErrors(errors, ['repeat']))}
			error={errors.repeat ?? null}
		/>
		<div class="flex justify-end gap-3 pt-2">
			<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
			<Button type="submit" disabled={saving}>{m.users_reset_submit()}</Button>
		</div>
	</form>
</BaseDialog>
