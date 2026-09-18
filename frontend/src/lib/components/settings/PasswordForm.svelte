<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { changePassword } from '$lib/api/auth';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { m } from '$lib/paraglide/messages';
	import {
		passwordErrorsFromApi,
		validateNewPassword,
		type PasswordErrors
	} from '$lib/settings/password';

	let current = $state('');
	let next = $state('');
	let repeat = $state('');
	let errors = $state<PasswordErrors>({});
	let saving = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errors = validateNewPassword(next, repeat);
		if (current === '') {
			errors = { ...errors, current: m.settings_password_current_required() };
		}
		if (Object.keys(errors).length > 0) {
			return;
		}
		saving = true;
		try {
			await changePassword(current, next);
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
	<Input
		id="current-password"
		label={m.settings_password_current()}
		type="password"
		autocomplete="current-password"
		bind:value={current}
		error={errors.current ?? null}
	/>
	<Input
		id="new-password"
		label={m.settings_password_new()}
		type="password"
		autocomplete="new-password"
		bind:value={next}
		error={errors.next ?? null}
	/>
	<Input
		id="repeat-password"
		label={m.settings_password_repeat()}
		type="password"
		autocomplete="new-password"
		bind:value={repeat}
		error={errors.repeat ?? null}
	/>
	<div class="flex justify-end">
		<Button type="submit" disabled={saving}>{m.settings_password_save()}</Button>
	</div>
</form>
