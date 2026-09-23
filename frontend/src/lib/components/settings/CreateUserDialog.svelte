<script lang="ts">
	import { Dialog, RadioGroup } from 'bits-ui';
	import Languages from 'lucide-svelte/icons/languages';
	import { toast } from 'svelte-sonner';
	import type { ColorUsage, Locale } from '$lib/api/auth';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import { createUser, type UserAccount, type UserRole } from '$lib/api/users';
	import { session } from '$lib/auth.svelte';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { languageOptions } from '$lib/i18n/languages';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import { passwordErrorsFromApi, validateNewPassword } from '$lib/settings/password';
	import { leastUsedColor, USER_COLORS, type UserColor } from '$lib/user/color';
	import ColorPicker from './ColorPicker.svelte';

	let {
		open = $bindable(false),
		usage,
		oncreated
	}: { open?: boolean; usage: ColorUsage[]; oncreated: (user: UserAccount) => void } = $props();

	let username = $state('');
	let displayName = $state('');
	let password = $state('');
	let role = $state<string>('user');
	let color = $state<UserColor>(USER_COLORS[0]);
	// Seeded from the language this admin is reading, not from the instance
	// default: someone setting up an account for the household almost always
	// speaks the language they are working in, whatever the instance was
	// configured with. The new member can change it themselves afterwards -
	// it is the one profile field nobody else may touch once the account
	// exists.
	// Held as a plain string because that is what the select binds; narrowed
	// back to Locale on submit, where the API type demands it.
	let locale = $state<string>(getLocale());
	let errors = $state<{ username?: string; password?: string }>({});
	let saving = $state(false);

	const localeOptions = languageOptions();

	// Only the instance owner hands out the admin role; the API answers 403
	// otherwise, and an option that always fails is worse than no option.
	const roles = $derived<{ value: UserRole; label: string; hint: string }[]>([
		{ value: 'user', label: m.users_role_member(), hint: m.users_role_member_hint() },
		...(session.user?.role === 'superadmin'
			? [
					{
						value: 'admin' as UserRole,
						label: m.users_role_admin(),
						hint: m.users_role_admin_hint()
					}
				]
			: [])
	]);

	// A fresh form every time the dialog opens.
	$effect(() => {
		if (open) {
			username = '';
			displayName = '';
			password = '';
			role = 'user';
			color = leastUsedColor(usage);
			locale = getLocale();
			errors = {};
		}
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errors = {};
		if (username.trim() === '') {
			errors.username = m.users_validation_username();
		}
		const pw = validateNewPassword(password, password);
		if (pw.next) {
			errors.password = pw.next;
		}
		if (Object.keys(errors).length > 0) {
			return;
		}
		saving = true;
		try {
			const created = await createUser({
				username: username.trim(),
				displayName: displayName.trim(),
				password,
				role: role as UserRole,
				color,
				locale: locale as Locale
			});
			toast.success(m.users_created({ username: created.username }));
			open = false;
			oncreated(created);
		} catch (error) {
			if (error instanceof ApiError && error.status === 409) {
				toast.error(m.users_username_taken());
			} else if (error instanceof ApiError && error.status === 422) {
				if (error.errors.some((e) => e.location === 'body.username')) {
					errors.username = m.users_validation_username();
				}
				const password = passwordErrorsFromApi(error.errors).next;
				if (password) {
					errors.password = password;
				}
				if (Object.keys(errors).length === 0) toast.error(error.detail ?? m.users_create_error());
			} else if (!isSignedOut(error)) {
				// 401 already redirects to the login page (see $lib/api/client).
				toast.error(m.users_create_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

<BaseDialog bind:open>
	<Dialog.Title class="font-display text-heading font-medium">
		{m.users_create_title()}
	</Dialog.Title>
	<Dialog.Description class="sr-only">{m.users_create_description()}</Dialog.Description>
	<form onsubmit={submit} class="mt-5 space-y-4">
		<Input
			id="new-user-name"
			label={m.login_username()}
			autocomplete="off"
			bind:value={username}
			error={errors.username ?? null}
		/>
		<!-- Optional: an empty one means the API keeps the login name, which is
		     exactly what a household of first names wants. -->
		<Input
			id="new-user-display-name"
			label={m.users_field_display_name_optional()}
			autocomplete="off"
			maxlength={64}
			counter={64}
			bind:value={displayName}
		/>
		<Input
			id="new-user-password"
			label={m.login_password()}
			type="password"
			autocomplete="new-password"
			bind:value={password}
			error={errors.password ?? null}
		/>
		<RadioGroup.Root
			bind:value={role}
			aria-label={m.users_field_role()}
			class="grid grid-cols-2 gap-3"
		>
			{#each roles as option (option.value)}
				<RadioGroup.Item
					value={option.value}
					class="flex items-start gap-3 rounded-md border border-border bg-surface-elevated px-4 py-3 text-left transition data-[state=checked]:border-[1.5px] data-[state=checked]:border-primary data-[state=checked]:bg-accent"
				>
					{#snippet children({ checked })}
						<span
							aria-hidden="true"
							class="mt-0.5 size-4 shrink-0 rounded-full border {checked
								? 'border-[5px] border-primary bg-surface'
								: 'border-border bg-surface-elevated'}"
						></span>
						<span class="flex flex-col">
							<span class="text-body-sm font-semibold">{option.label}</span>
							<span class="text-micro text-text-muted">{option.hint}</span>
						</span>
					{/snippet}
				</RadioGroup.Item>
			{/each}
		</RadioGroup.Root>
		<ColorPicker bind:value={color} {usage} label={m.users_field_color()} />
		<div>
			<span class="block text-caption font-semibold">{m.users_field_language()}</span>
			<p class="mt-1.5 mb-3 text-micro text-text-muted">{m.users_field_language_hint()}</p>
			<Select
				bind:value={locale}
				options={localeOptions}
				label={m.users_field_language()}
				class="w-full bg-surface-elevated"
			>
				{#snippet icon()}
					<Languages class="size-4 shrink-0 text-text-muted" aria-hidden="true" />
				{/snippet}
			</Select>
		</div>
		<div class="flex justify-end gap-3 pt-2">
			<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
			<Button type="submit" disabled={saving}>{m.users_create_submit()}</Button>
		</div>
	</form>
</BaseDialog>
