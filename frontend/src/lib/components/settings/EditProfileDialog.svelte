<script lang="ts">
	import { untrack } from 'svelte';
	import Check from '@lucide/svelte/icons/check';
	import MailQuestionMark from '@lucide/svelte/icons/mail-question-mark';
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import {
		removeUserAvatar,
		setUserAvatar,
		updateUser,
		type PersonEntry,
		type UserAccount
	} from '$lib/api/users';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { ColorUsage } from '$lib/api/auth';
	import type { UserColor } from '$lib/user/color';
	import AvatarControl from './AvatarControl.svelte';
	import ColorPicker from './ColorPicker.svelte';

	let {
		open = $bindable(false),
		user,
		usage,
		editProfile,
		manageAccount,
		email = '',
		emailVerified = false,
		mailEnabled = false,
		onsaved
	}: {
		open?: boolean;
		user: PersonEntry;
		usage: ColorUsage[];
		/** Photo, display name and color - the owner's alone. */
		editProfile: boolean;
		/** The address - the same rank rule as a reset. */
		manageAccount: boolean;
		email?: string;
		emailVerified?: boolean;
		/** Whether a changed address gets a confirmation mail. */
		mailEnabled?: boolean;
		/** The account as saved; a PATCH answer carries the address too. */
		onsaved: (user: PersonEntry | UserAccount) => void;
	} = $props();

	// The field's id has to be unique per mounted dialog.
	const uid = $props.id();

	let displayName = $state('');
	let color = $state<UserColor>('amber');
	let address = $state('');
	let emailError = $state<string | null>(null);
	let saving = $state(false);

	const typed = $derived(address.trim());
	const emailChanged = $derived(typed !== email);

	// A fresh form every time the dialog opens, seeded from the row. Only
	// `open` is tracked: a picture set while the dialog is open replaces
	// `user`, and re-seeding then would wipe the fields being edited.
	$effect(() => {
		if (open) {
			untrack(() => {
				displayName = user.displayName;
				color = user.color;
				address = email;
				emailError = null;
				saving = false;
			});
		}
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		// Only what changed: an admin's request must not carry a name or a
		// color, which the API refuses them.
		const patch: Parameters<typeof updateUser>[1] = {};
		if (editProfile && displayName.trim() !== user.displayName) {
			patch.displayName = displayName.trim();
		}
		if (editProfile && color !== user.color) patch.color = color;
		if (manageAccount && emailChanged) patch.email = typed;
		if (Object.keys(patch).length === 0) {
			open = false;
			return;
		}
		saving = true;
		emailError = null;
		try {
			const saved = await updateUser(user.id, patch);
			toast.success(m.users_profile_saved());
			open = false;
			onsaved(saved);
		} catch (error) {
			if (
				error instanceof ApiError &&
				error.status === 422 &&
				error.errors.some((e) => e.location === 'body.email')
			) {
				emailError = m.settings_profile_email_invalid();
			} else if (error instanceof ApiError && error.status === 403) {
				toast.error(editProfile ? m.users_profile_forbidden() : m.users_rank_required());
			} else if (!isSignedOut(error)) {
				// 401 already redirects to the login page (see $lib/api/client).
				toast.error(m.users_profile_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

<BaseDialog bind:open>
	<Dialog.Title class="font-display text-heading font-medium">
		{m.users_edit_profile_title({ username: user.username })}
	</Dialog.Title>
	<Dialog.Description class="sr-only">
		{editProfile ? m.users_edit_profile_description() : m.users_edit_email_description()}
	</Dialog.Description>
	{#if editProfile}
		<div class="mt-5">
			<AvatarControl
				person={user}
				upload={(file, crop) => setUserAvatar(user.id, file, crop)}
				remove={() => removeUserAvatar(user.id)}
				onchange={(avatarId) => onsaved({ ...user, avatarId })}
			/>
		</div>
	{/if}
	<form onsubmit={submit} class="mt-5 space-y-4">
		{#if editProfile}
			<!-- `maxlength` mirrors the API's 64-rune limit. -->
			<Input
				id="{uid}-display-name"
				label={m.users_field_display_name()}
				autocomplete="off"
				maxlength={64}
				counter={64}
				bind:value={displayName}
			/>
			<ColorPicker bind:value={color} {usage} label={m.users_field_color()} />
		{/if}
		{#if manageAccount}
			<!-- The state line the row and the sheet show, while the field
			     still holds the saved address; what happens to a new one once
			     it is changed. -->
			<div class="space-y-1.5">
				<Input
					id="{uid}-email"
					label={m.users_field_email()}
					type="email"
					autocomplete="off"
					maxlength={254}
					bind:value={address}
					oninput={() => (emailError = null)}
					error={emailError}
					hint={emailChanged && typed
						? mailEnabled
							? m.users_edit_email_mail_hint()
							: m.users_edit_email_no_mail_hint()
						: undefined}
				/>
				{#if email && !emailChanged}
					<p class="flex items-center gap-1 text-micro text-text-muted">
						{#if emailVerified}
							<Check class="size-3.5 shrink-0" aria-hidden="true" />
							{m.settings_profile_email_verified()}
						{:else}
							<MailQuestionMark class="size-3.5 shrink-0" aria-hidden="true" />
							{m.settings_profile_email_unconfirmed()}
						{/if}
					</p>
				{/if}
			</div>
		{/if}
		<div class="flex justify-end gap-3 pt-2">
			<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
			<Button type="submit" disabled={saving}>{m.common_save()}</Button>
		</div>
	</form>
</BaseDialog>
