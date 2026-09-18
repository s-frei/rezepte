<script lang="ts">
	import { onMount } from 'svelte';
	import Plus from 'lucide-svelte/icons/plus';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import {
		deleteUser,
		listUsers,
		updateUser,
		type UserAccount,
		type UserRole
	} from '$lib/api/users';
	import { session } from '$lib/auth.svelte';
	import CreateUserDialog from '$lib/components/settings/CreateUserDialog.svelte';
	import ResetPasswordDialog from '$lib/components/settings/ResetPasswordDialog.svelte';
	import SettingsLayout from '$lib/components/settings/SettingsLayout.svelte';
	import UserTable from '$lib/components/settings/UserTable.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import { m } from '$lib/paraglide/messages';

	let users = $state<UserAccount[]>([]);
	let loading = $state(true);
	let loadFailed = $state(false);
	let createOpen = $state(false);
	let resetOpen = $state(false);
	let resetTarget = $state<UserAccount | null>(null);
	let deleteOpen = $state(false);
	let deleteTarget = $state<UserAccount | null>(null);

	// An inline error with a retry button instead of a toast over an empty
	// list: an admin cannot tell a failed load from "no users", and a toast
	// leaves no way back onto the list short of reloading the page.
	async function load() {
		loading = true;
		loadFailed = false;
		try {
			users = await listUsers();
		} catch (error) {
			// On a 401 the client is already navigating to the login page.
			loadFailed = !isSignedOut(error);
		} finally {
			loading = false;
		}
	}

	onMount(load);

	function replace(updated: UserAccount) {
		users = users.map((u) => (u.id === updated.id ? updated : u));
	}

	async function changeRole(user: UserAccount, role: UserRole) {
		try {
			replace(await updateUser(user.id, { role }));
			toast.success(m.users_role_changed({ username: user.username }));
		} catch (error) {
			if (error instanceof ApiError && error.status === 409) {
				toast.error(m.users_last_admin_role());
			} else if (!isSignedOut(error)) {
				toast.error(m.users_update_error());
			}
			throw error; // lets UserRow snap its select back
		}
	}

	function askReset(user: UserAccount) {
		resetTarget = user;
		resetOpen = true;
	}

	function askDelete(user: UserAccount) {
		deleteTarget = user;
		deleteOpen = true;
	}

	async function remove() {
		const target = deleteTarget;
		if (!target) return;
		try {
			await deleteUser(target.id);
			users = users.filter((u) => u.id !== target.id);
			toast.success(m.users_deleted({ username: target.username }));
		} catch (error) {
			if (error instanceof ApiError && error.status === 409) {
				toast.error(m.users_delete_conflict());
			} else if (!isSignedOut(error)) {
				toast.error(m.users_delete_error());
			}
		}
	}
</script>

<svelte:head><title>{m.settings_nav_users()} · {m.app_name()}</title></svelte:head>

<SettingsLayout active="users">
	<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-users">
		<!-- Side by side the heading and the button need 306px, 4px more than a
		     360px phone leaves inside this card, and the label wrapped into two
		     lines. Below `md` the button gets its own full-width row instead. -->
		<div class="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between md:gap-4">
			<h2 id="settings-users" class="font-display text-heading font-medium">
				{m.settings_nav_users()}
			</h2>
			<Button
				variant="accent"
				onclick={() => (createOpen = true)}
				class="w-full whitespace-nowrap md:w-auto"
			>
				<Plus class="size-4" aria-hidden="true" />
				{m.users_create()}
			</Button>
		</div>
		{#if loading}
			<div class="space-y-3">
				<Skeleton class="h-12 rounded-md" />
				<Skeleton class="h-12 rounded-md" />
			</div>
		{:else if loadFailed}
			<div class="flex flex-col items-center gap-4 py-10 text-center">
				<p class="text-body text-text-muted">{m.users_load_error()}</p>
				<Button variant="secondary" onclick={load}>{m.common_retry()}</Button>
			</div>
		{:else}
			<UserTable
				{users}
				meId={session.user?.id ?? ''}
				onrole={changeRole}
				onreset={askReset}
				ondelete={askDelete}
			/>
		{/if}
	</section>
</SettingsLayout>

<CreateUserDialog
	bind:open={createOpen}
	oncreated={(user) =>
		(users = [...users, user].sort((a, b) => a.username.localeCompare(b.username, 'de')))}
/>
<!-- Stays mounted and keeps its target after closing so the close transition
     can play; `askReset` replaces the target on the next open. -->
<ResetPasswordDialog bind:open={resetOpen} user={resetTarget} />
<ConfirmDialog
	bind:open={deleteOpen}
	title={m.users_delete_confirm_title()}
	text={m.users_delete_confirm_text({ username: deleteTarget?.username ?? '' })}
	confirmLabel={m.common_delete()}
	destructive
	onconfirm={remove}
/>
