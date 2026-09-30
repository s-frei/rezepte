<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { changePassword } from '$lib/api/auth';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import { session } from '$lib/auth.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { withoutErrors } from '$lib/form-errors';
	import { m } from '$lib/paraglide/messages';
	import {
		PASSWORD_MIN,
		passwordErrorsFromApi,
		validateNewPassword,
		type PasswordErrors
	} from '$lib/settings/password';
	import PasswordStrength from './PasswordStrength.svelte';

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
		} catch (error) {
			if (error instanceof ApiError && error.status === 422) {
				errors = passwordErrorsFromApi(error.errors);
				if (Object.keys(errors).length === 0) {
					toast.error(error.detail ?? m.settings_password_error());
				}
			} else if (!isSignedOut(error)) {
				// 401 already redirects to the login page (see $lib/api/client).
				toast.error(m.settings_password_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

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
	<div class="space-y-1.5">
		<Input
			id="new-password"
			label={m.settings_password_new()}
			type="password"
			autocomplete="new-password"
			bind:value={next}
			oninput={() => (errors = withoutErrors(errors, ['next', 'repeat']))}
			error={errors.next ?? null}
			hint={m.settings_password_too_short({ min: PASSWORD_MIN })}
		/>
		<PasswordStrength
			password={next}
			userInputs={[session.user?.username ?? '', session.user?.displayName ?? '']}
		/>
	</div>
	<Input
		id="repeat-password"
		label={m.settings_password_repeat()}
		type="password"
		autocomplete="new-password"
		bind:value={repeat}
		oninput={() => (errors = withoutErrors(errors, ['repeat']))}
		error={errors.repeat ?? null}
	/>
	<div class="flex justify-end">
		<Button type="submit" disabled={saving}>{m.settings_password_save()}</Button>
	</div>
</form>
