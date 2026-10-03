<script lang="ts">
	import { Dialog, RadioGroup } from 'bits-ui';
	import Languages from '@lucide/svelte/icons/languages';
	import { toast } from 'svelte-sonner';
	import type { ColorUsage, Locale } from '$lib/api/auth';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import { createUser, type SetupLinkInfo, type UserAccount, type UserRole } from '$lib/api/users';
	import { session } from '$lib/auth.svelte';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { languageOptions } from '$lib/i18n/languages';
	import { withoutErrors } from '$lib/form-errors';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import { PASSWORD_MIN, passwordErrorsFromApi, validateNewPassword } from '$lib/settings/password';
	import { leastUsedColor, USER_COLORS, type UserColor } from '$lib/user/color';
	import ColorPicker from './ColorPicker.svelte';
	import PasswordStrength from './PasswordStrength.svelte';

	let {
		open = $bindable(false),
		usage,
		mailEnabled = false,
		oncreated
	}: {
		open?: boolean;
		usage: ColorUsage[];
		/** Whether Rezepte sends mail: the setup link then goes to the address given. */
		mailEnabled?: boolean;
		oncreated: (user: UserAccount, setupLink: SetupLinkInfo | null) => void;
	} = $props();

	// "link" sends a one-time setup link and never shows the admin the
	// password; "password" is the old form, for the rare case the admin
	// hands the account over in person. Link first, since not knowing the
	// password is the point.
	let mode = $state<'link' | 'password'>('link');
	let username = $state('');
	let displayName = $state('');
	// Profile, not sign-in: it sits with the person in either mode, and is
	// where the setup link goes once mail is on.
	let email = $state('');
	let password = $state('');
	// Typed twice, like the reset and the own change: the admin hands this
	// password on, and a typo nobody saw would lock the new member out.
	let repeat = $state('');
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
	let errors = $state<{ username?: string; email?: string; password?: string; repeat?: string }>(
		{}
	);
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
			mode = 'link';
			username = '';
			displayName = '';
			email = '';
			password = '';
			repeat = '';
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
		if (mode === 'password') {
			const pw = validateNewPassword(password, repeat);
			if (pw.next) {
				errors.password = pw.next;
			}
			if (pw.repeat) {
				errors.repeat = pw.repeat;
			}
		}
		if (Object.keys(errors).length > 0) {
			return;
		}
		saving = true;
		try {
			const created = await createUser({
				username: username.trim(),
				displayName: displayName.trim(),
				password: mode === 'password' ? password : undefined,
				role: role as UserRole,
				color,
				locale: locale as Locale,
				email: email.trim() || undefined
			});
			toast.success(m.users_created({ username: created.username }));
			open = false;
			oncreated(created, created.setupLink ?? null);
		} catch (error) {
			if (error instanceof ApiError && error.status === 409) {
				toast.error(m.users_username_taken());
			} else if (error instanceof ApiError && error.status === 422) {
				if (error.errors.some((e) => e.location === 'body.username')) {
					errors.username = m.users_validation_username();
				}
				if (error.errors.some((e) => e.location === 'body.email')) {
					errors.email = m.settings_profile_email_invalid();
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

{#snippet section(title: string)}
	<!-- The people list's group heading (italic display face in primary),
	     run out to the column's edge by the contents sheet's dotted leader,
	     so each heading visibly owns its column. -->
	<div class="flex items-baseline gap-3">
		<h3 class="font-display text-body-lg font-medium text-primary italic">{title}</h3>
		<span aria-hidden="true" class="min-w-6 flex-1 border-b-2 border-dotted border-border"></span>
	</div>
{/snippet}

<!-- Wide, in two columns from md: stacked, its eleven fields outgrow a
     720px-high laptop screen and scroll in a box with room to spare on
     both sides. Who the person is on the left - the same fields Edit
     profile has - and what they may do and how they get in on the right,
     where the password fields grow when that mode is picked. On a phone
     the two stack in that order. -->
<BaseDialog bind:open wide>
	<Dialog.Title class="font-display text-heading font-medium">
		{m.users_create_title()}
	</Dialog.Title>
	<Dialog.Description class="sr-only">{m.users_create_description()}</Dialog.Description>
	<form onsubmit={submit} class="mt-5 space-y-4">
		<div class="grid gap-x-6 gap-y-8 md:grid-cols-2">
			<div class="space-y-4">
				{@render section(m.users_create_section_person())}
				<Input
					id="new-user-name"
					label={m.login_username()}
					autocomplete="off"
					required
					bind:value={username}
					oninput={() => (errors = withoutErrors(errors, ['username']))}
					error={errors.username ?? null}
				/>
				<!-- Optional: an empty one means the API keeps the login name, which is
				     exactly what a household of first names wants. -->
				<Input
					id="new-user-display-name"
					label={m.users_field_display_name()}
					autocomplete="off"
					maxlength={64}
					counter={64}
					bind:value={displayName}
				/>
				<Input
					id="new-user-email"
					type="email"
					label="{m.users_field_email()} · {m.users_field_optional()}"
					hint={m.users_field_email_hint()}
					autocomplete="off"
					maxlength={254}
					bind:value={email}
					oninput={() => (errors = withoutErrors(errors, ['email']))}
					error={errors.email ?? null}
				/>
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
			</div>
			<div class="space-y-4">
				{@render section(m.users_create_section_access())}
				<div>
					<span id="new-user-role-label" class="mb-3 block text-caption font-semibold">
						{m.users_field_role()}
					</span>
					<RadioGroup.Root
						bind:value={role}
						aria-labelledby="new-user-role-label"
						class="grid grid-cols-1 overflow-hidden rounded-md border border-border"
					>
						{#each roles as option (option.value)}
							<RadioGroup.Item
								value={option.value}
								class="flex items-start gap-3 border-border bg-surface-elevated px-4 py-3 text-left transition not-first:border-t data-[state=checked]:bg-accent"
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
				</div>
				<div>
					<span id="new-user-sign-in-label" class="block text-caption font-semibold">
						{m.users_field_sign_in()}
					</span>
					<p class="mt-1.5 mb-3 text-micro text-text-muted">
						{mode === 'link'
							? m.users_create_mode_link_hint()
							: m.users_create_mode_password_hint()}
					</p>
					<RadioGroup.Root
						bind:value={mode}
						aria-labelledby="new-user-sign-in-label"
						class="grid grid-cols-1 overflow-hidden rounded-md border border-border"
					>
						{#each [{ value: 'link' as const, label: m.users_create_mode_link() }, { value: 'password' as const, label: m.users_create_mode_password() }] as option (option.value)}
							<RadioGroup.Item
								value={option.value}
								class="flex items-center gap-2 border-border bg-surface-elevated px-4 py-3 text-left text-body-sm font-semibold transition not-first:border-t data-[state=checked]:bg-accent"
							>
								{#snippet children({ checked })}
									<span
										aria-hidden="true"
										class="size-4 shrink-0 rounded-full border {checked
											? 'border-[5px] border-primary bg-surface'
											: 'border-border bg-surface-elevated'}"
									></span>
									{option.label}
								{/snippet}
							</RadioGroup.Item>
						{/each}
					</RadioGroup.Root>
				</div>
				{#if mode === 'password'}
					<div class="space-y-1.5">
						<Input
							id="new-user-password"
							label={m.login_password()}
							type="password"
							autocomplete="new-password"
							required
							bind:value={password}
							oninput={() => (errors = withoutErrors(errors, ['password', 'repeat']))}
							error={errors.password ?? null}
							hint={m.settings_password_too_short({ min: PASSWORD_MIN })}
						/>
						<PasswordStrength {password} userInputs={[username, displayName]} />
					</div>
					<Input
						id="new-user-password-repeat"
						label={m.settings_password_repeat()}
						type="password"
						autocomplete="new-password"
						required
						bind:value={repeat}
						oninput={() => (errors = withoutErrors(errors, ['repeat']))}
						error={errors.repeat ?? null}
					/>
				{/if}
			</div>
		</div>
		<div class="flex justify-end gap-3 pt-2">
			<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
			<Button type="submit" disabled={saving}>
				{mode === 'link' && mailEnabled && email.trim()
					? m.users_create_submit_and_send()
					: m.users_create_submit()}
			</Button>
		</div>
	</form>
</BaseDialog>
