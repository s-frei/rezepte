<script lang="ts">
	import KeyRound from '@lucide/svelte/icons/key-round';
	import Link from '@lucide/svelte/icons/link';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import { tick } from 'svelte';
	import type { PersonEntry, UserRole } from '$lib/api/users';
	import BottomSheet from '$lib/components/ui/BottomSheet.svelte';
	import SegmentedControl from '$lib/components/ui/SegmentedControl.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/recipe/format';
	import { roleLabel } from '$lib/roles';
	import { focusManageButton } from '$lib/settings/person-focus';
	import { rowPermissions } from '$lib/settings/row-permissions';
	import PersonCard from '$lib/components/ui/PersonCard.svelte';

	let {
		open = $bindable(false),
		person,
		actorRole,
		isSelf,
		canShare,
		hasPassword,
		hasIdentity,
		setupLinkExpiresAt,
		onrole,
		onshare,
		onedit,
		onreset,
		onsetuplink,
		onrevokelink,
		ondelete
	}: {
		open?: boolean;
		person: PersonEntry;
		/** The signed-in user's role; decides which actions the sheet offers. */
		actorRole: UserRole;
		isSelf: boolean;
		/** Whether this person may share publicly; undefined where the viewer
		 * cannot see it. */
		canShare?: boolean;
		/** Admin-only, like `canShare`: undefined where the viewer cannot see it. */
		hasPassword?: boolean;
		/** Admin-only: whether the person connected an identity provider. */
		hasIdentity?: boolean;
		/** Admin-only; set while this person has an open setup link. */
		setupLinkExpiresAt?: string;
		/** Rejects when the API refused; the control then snaps back. */
		onrole: (person: PersonEntry, role: UserRole) => Promise<void>;
		/** Rejects when the API refused; the switch then snaps back. */
		onshare: (person: PersonEntry, on: boolean) => Promise<void>;
		onedit: (person: PersonEntry) => void;
		onreset: (person: PersonEntry) => void;
		/** Issues a fresh setup link and opens the dialog that shows it. */
		onsetuplink: (person: PersonEntry) => void;
		/** Revokes the open setup link. */
		onrevokelink: (person: PersonEntry) => void;
		ondelete: (person: PersonEntry) => void;
	} = $props();

	const permissions = $derived(rowPermissions(actorRole, person, isSelf));
	const setupStatus = $derived(
		!permissions.manageAccount
			? null
			: setupLinkExpiresAt
				? m.users_setup_link_open({ date: formatDate(setupLinkExpiresAt) })
				: !hasPassword && !hasIdentity
					? m.users_not_set_up()
					: null
	);

	// Writable derived: follows the list, so a successful change shows the new
	// role, and snaps back on its own when the API refuses one.
	let role = $derived<UserRole>(person.role);
	const roleOptions: { value: UserRole; label: string }[] = [
		{ value: 'admin', label: m.users_role_admin() },
		{ value: 'user', label: m.users_role_member() }
	];

	// Follows the list like `role`, and snaps back when the API refuses.
	let sharing = $derived<boolean>(canShare ?? false);
	const showSharing = $derived(permissions.toggleSharing && canShare !== undefined);

	async function toggleSharing(next: boolean) {
		sharing = next;
		try {
			await onshare(person, next);
		} catch {
			sharing = canShare ?? false;
		}
	}

	// One change at a time: the control's arrow keys move and check in one
	// step, and every step would otherwise be a request of its own, their
	// answers free to arrive out of order.
	let pending = false;

	async function changeRole(next: UserRole) {
		if (pending) {
			role = person.role;
			return;
		}
		pending = true;
		try {
			await onrole(person, next);
		} catch {
			role = person.role;
		} finally {
			pending = false;
		}
	}

	// The sheet closes before the next dialog opens, the way the contents
	// sheet hands over to a page: two sheets never stand on top of each other.
	// Focus goes to the row first, so that is where the next dialog returns
	// it - not to a button in a sheet that is gone by then.
	let handingOver = false;

	async function hand(action: (person: PersonEntry) => void) {
		handingOver = true;
		open = false;
		// A tick, so the sheet has let go of its focus trap: moved while the
		// trap still holds, focus would be pulled straight back into the sheet.
		await tick();
		focusManageButton(person.id);
		action(person);
	}

	// Closing hands focus back to the row that opened the sheet, found by id:
	// after a role change that row is a new element in another group, and the
	// one the sheet remembers no longer exists. A handover has already placed
	// focus, and the next dialog holds it.
	function returnFocus(event: Event) {
		event.preventDefault();
		if (!handingOver) focusManageButton(person.id);
		handingOver = false;
	}

	const row =
		'flex min-h-12 w-full items-center gap-3 rounded-md px-3 py-3 text-left text-body font-medium transition hover:bg-surface focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary';
</script>

<!--
	A phone's way into one person: the row it opens from names them, the sheet
	carries what can be done - the same actions a wide screen puts in the row,
	stacked as full-width lines a thumb can hit. Shaped like the contents
	sheet, so the phone has one kind of sheet.
-->
<BottomSheet bind:open closeLabel={m.common_close()} onCloseAutoFocus={returnFocus}>
	<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">
		<PersonCard {person} asTitle />
		{#if setupStatus}
			<p class="mt-2 flex flex-wrap items-center gap-x-2 text-micro text-text-muted">
				<span>{setupStatus}</span>
				{#if setupLinkExpiresAt}
					<button
						type="button"
						onclick={() => onrevokelink(person)}
						class="font-semibold text-text underline decoration-dotted underline-offset-2"
					>
						{m.users_setup_link_revoke()}
					</button>
				{/if}
			</p>
		{/if}

		<!-- A surface panel, as on the settings cards: the segmented
				control's track is the background color and vanishes on a
				sheet that is the background color too, leaving "Admin" to
				read as one more action line. -->
		<div class="mt-6 rounded-xl bg-surface p-4">
			<p class="mb-2 text-caption text-text-muted">{m.users_field_role()}</p>
			{#if permissions.changeRole}
				<SegmentedControl
					bind:value={role}
					options={roleOptions}
					label={m.users_role_aria({ username: person.username })}
					onchange={changeRole}
				/>
			{:else}
				<p class="text-body font-medium">{roleLabel(person.role)}</p>
			{/if}
			{#if showSharing}
				<!-- A setting on the person, like the role above, so it
					     sits in the same panel rather than among the actions. -->
				<div class="mt-4 flex items-center justify-between gap-3 border-t border-border pt-4">
					<span class="text-body font-medium">{m.users_share_toggle()}</span>
					<Switch checked={sharing} label={m.users_share_toggle()} onchange={toggleSharing} />
				</div>
			{/if}
		</div>

		<div class="-mx-3 mt-5 border-t border-border pt-2">
			{#if permissions.editProfile}
				<button type="button" class={row} onclick={() => hand(onedit)}>
					<Pencil class="size-4 shrink-0 text-text-muted" aria-hidden="true" />
					{m.users_edit_profile()}
				</button>
			{/if}
			{#if permissions.manageAccount}
				<button type="button" class={row} onclick={() => hand(onreset)}>
					<KeyRound class="size-4 shrink-0 text-text-muted" aria-hidden="true" />
					{m.users_reset_password()}
				</button>
				<button type="button" class={row} onclick={() => hand(onsetuplink)}>
					<Link class="size-4 shrink-0 text-text-muted" aria-hidden="true" />
					{m.users_setup_link_action()}
				</button>
				<button type="button" class="{row} text-destructive" onclick={() => hand(ondelete)}>
					<Trash2 class="size-4 shrink-0" aria-hidden="true" />
					{m.users_delete_account()}
				</button>
			{/if}
		</div>
	</div>
</BottomSheet>
