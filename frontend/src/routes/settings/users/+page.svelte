<script lang="ts">
	import { onMount, tick } from 'svelte';
	import Plus from '@lucide/svelte/icons/plus';
	import { toast } from 'svelte-sonner';
	import { listColorUsage, type ColorUsage } from '$lib/api/auth';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import {
		deleteUser,
		listPeople,
		updateUser,
		type PersonEntry,
		type UserRole
	} from '$lib/api/users';
	import { session } from '$lib/auth.svelte';
	import CreateUserDialog from '$lib/components/settings/CreateUserDialog.svelte';
	import LinkPreviewsCard from '$lib/components/settings/LinkPreviewsCard.svelte';
	import RecipeEditingCard from '$lib/components/settings/RecipeEditingCard.svelte';
	import ResetPasswordDialog from '$lib/components/settings/ResetPasswordDialog.svelte';
	import SettingsLayout from '$lib/components/settings/SettingsLayout.svelte';
	import PeopleList from '$lib/components/settings/PeopleList.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import { m } from '$lib/paraglide/messages';
	import { isAdminRole, roleLabel } from '$lib/roles';

	let users = $state<PersonEntry[]>([]);
	let usage = $state<ColorUsage[]>([]);
	let loading = $state(true);
	let loadFailed = $state(false);
	let createOpen = $state(false);
	let resetOpen = $state(false);
	let resetTarget = $state<PersonEntry | null>(null);
	let deleteOpen = $state(false);
	let deleteTarget = $state<PersonEntry | null>(null);

	// Every account reads this page; only an admin gets the controls, the
	// pickers that need the color counts, and the household's editing card.
	const isAdmin = $derived(isAdminRole(session.user?.role));

	// Advisory, like on the profile page: the counts only mark a color in the
	// pickers, so a failure leaves them unmarked instead of failing the page.
	// Refreshed after every write that can move a color, because a stale
	// count marks one nobody holds any more.
	async function loadUsage() {
		try {
			usage = await listColorUsage();
		} catch {
			usage = [];
		}
	}

	// An inline error with a retry button instead of a toast over an empty
	// list: nobody can tell a failed load from "no one here", and a toast
	// leaves no way back onto the list short of reloading the page.
	async function load() {
		loading = true;
		loadFailed = false;
		try {
			const [list] = await Promise.all([listPeople(), isAdmin ? loadUsage() : undefined]);
			users = list;
		} catch (error) {
			// On a 401 the client is already navigating to the login page.
			loadFailed = !isSignedOut(error);
		} finally {
			loading = false;
		}
	}

	onMount(load);

	function replace(updated: PersonEntry) {
		users = users.map((u) => (u.id === updated.id ? updated : u));
	}

	function profileSaved(updated: PersonEntry) {
		replace(updated);
		void loadUsage();
	}

	function userCreated(user: PersonEntry) {
		users = [...users, user];
		void loadUsage();
	}

	// A role takes effect with one tap and carries rights, so the toast names
	// the new role and offers the way back: a stray tap costs a second tap,
	// not a trip into the sheet to find the old role again.
	async function changeRole(user: PersonEntry, role: UserRole) {
		const previous = user.role;
		try {
			const updated = await updateUser(user.id, { role });
			replace(updated);
			toast.success(m.users_role_changed({ name: updated.displayName, role: roleLabel(role) }), {
				action: {
					label: m.common_undo(),
					// changeRole raises its own toast on failure; the rethrow is
					// only for the control that made the change.
					onClick: () => changeRole(updated, previous).catch(() => {})
				}
			});
		} catch (error) {
			if (error instanceof ApiError && error.status === 409) {
				toast.error(m.users_owner_protected());
			} else if (error instanceof ApiError && error.status === 403) {
				toast.error(m.users_rank_required());
			} else if (!isSignedOut(error)) {
				toast.error(m.users_update_error());
			}
			throw error; // lets the row or the sheet snap its control back
		}
	}

	function askReset(user: PersonEntry) {
		resetTarget = user;
		resetOpen = true;
	}

	function askDelete(user: PersonEntry) {
		deleteTarget = user;
		deleteOpen = true;
	}

	async function remove() {
		const target = deleteTarget;
		if (!target) return;
		try {
			await deleteUser(target.id);
			users = users.filter((u) => u.id !== target.id);
			void loadUsage();
			// The dialog hands focus back to the row it was opened from, and that
			// row is gone now: without a new home focus falls to <body> and a
			// screen reader starts over at the top. The card's heading is the
			// nearest place that still stands.
			await tick();
			if (!document.activeElement || document.activeElement === document.body) {
				document.getElementById('settings-users')?.focus();
			}
			toast.success(m.users_deleted({ username: target.username }));
		} catch (error) {
			if (error instanceof ApiError && error.status === 409) {
				toast.error(m.users_delete_conflict());
			} else if (error instanceof ApiError && error.status === 403) {
				toast.error(m.users_rank_required());
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
			<h2
				id="settings-users"
				tabindex="-1"
				class="font-display text-heading font-medium outline-none"
			>
				{m.settings_nav_users()}
			</h2>
			{#if isAdmin}
				<Button
					variant="accent"
					onclick={() => (createOpen = true)}
					class="w-full whitespace-nowrap md:w-auto"
				>
					<Plus class="size-4" aria-hidden="true" />
					{m.users_create()}
				</Button>
			{/if}
		</div>
		{#if !isAdmin}
			<p class="mb-4 text-body text-text-muted">{m.users_list_hint()}</p>
		{/if}
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
			<PeopleList
				people={users}
				meId={session.user?.id ?? ''}
				actorRole={session.user?.role ?? 'user'}
				{usage}
				onrole={changeRole}
				onreset={askReset}
				ondelete={askDelete}
				onprofile={profileSaved}
			/>
		{/if}
	</section>
	{#if isAdmin}
		<RecipeEditingCard ownerName={users.find((u) => u.role === 'superadmin')?.displayName ?? ''} />
		<LinkPreviewsCard ownerName={users.find((u) => u.role === 'superadmin')?.displayName ?? ''} />
	{/if}
</SettingsLayout>

{#if isAdmin}
	<!-- The new account is appended, not sorted in: PeopleList owns the display
	     order and drops the row into its group. -->
	<CreateUserDialog bind:open={createOpen} {usage} oncreated={userCreated} />
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
{/if}
