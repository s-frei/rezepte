<script lang="ts" generics="E extends { next?: string; repeat?: string }">
	import Input from '#lib/components/ui/Input.svelte';
	import { withoutErrors } from '#lib/form-errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { PASSWORD_MIN } from '#lib/settings/password.js';
	import PasswordStrength from './PasswordStrength.svelte';

	// The new password, its strength and the repeat: the same three fields
	// wherever a password is set. The repeat field's id is `{id}-repeat`.
	let {
		id,
		label = m.settings_password_new(),
		password = $bindable(''),
		repeat = $bindable(''),
		errors = $bindable(),
		userInputs,
		required = false,
		floating = false
	}: {
		id: string;
		label?: string;
		password?: string;
		repeat?: string;
		errors: E;
		/** Names the strength meter counts against the password. */
		userInputs: string[];
		required?: boolean;
		floating?: boolean;
	} = $props();
</script>

<div class="space-y-1.5">
	<Input
		{id}
		{label}
		type="password"
		autocomplete="new-password"
		{required}
		{floating}
		bind:value={password}
		oninput={() => (errors = withoutErrors(errors, ['next', 'repeat']))}
		error={errors.next ?? null}
		hint={m.settings_password_too_short({ min: PASSWORD_MIN })}
	/>
	<PasswordStrength {password} {userInputs} />
</div>
<Input
	id="{id}-repeat"
	label={m.settings_password_repeat()}
	type="password"
	autocomplete="new-password"
	{required}
	{floating}
	bind:value={repeat}
	oninput={() => (errors = withoutErrors(errors, ['repeat']))}
	error={errors.repeat ?? null}
/>
