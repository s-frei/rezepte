<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import { updateUser, type UserAccount } from '$lib/api/users';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { ColorUsage } from '$lib/api/auth';
	import type { UserColor } from '$lib/user/color';
	import ColorPicker from './ColorPicker.svelte';

	let {
		open = $bindable(false),
		user,
		usage,
		onsaved
	}: {
		open?: boolean;
		user: UserAccount;
		usage: ColorUsage[];
		onsaved: (user: UserAccount) => void;
	} = $props();

	// One dialog per row, so the field's id has to be unique per instance.
	const uid = $props.id();

	let displayName = $state('');
	let color = $state<UserColor>('amber');
	let saving = $state(false);

	// A fresh form every time the dialog opens, seeded from the row.
	$effect(() => {
		if (open) {
			displayName = user.displayName;
			color = user.color;
			saving = false;
		}
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		saving = true;
		try {
			const saved = await updateUser(user.id, { displayName: displayName.trim(), color });
			toast.success(m.users_profile_saved());
			open = false;
			onsaved(saved);
		} catch (error) {
			if (error instanceof ApiError && error.status === 403) {
				toast.error(m.users_profile_forbidden());
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
	<Dialog.Description class="sr-only">{m.users_edit_profile_description()}</Dialog.Description>
	<form onsubmit={submit} class="mt-5 space-y-4">
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
		<div class="flex justify-end gap-3 pt-2">
			<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
			<Button type="submit" disabled={saving}>{m.common_save()}</Button>
		</div>
	</form>
</BaseDialog>
