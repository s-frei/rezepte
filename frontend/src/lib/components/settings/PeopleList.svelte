<script lang="ts">
	import { tick } from 'svelte';
	import type { ColorUsage } from '$lib/api/auth';
	import type { PersonEntry, UserAccount, UserRole } from '$lib/api/users';
	import { m } from '$lib/paraglide/messages';
	import { groupPeople } from '$lib/settings/people-groups';
	import { rowPermissions } from '$lib/settings/row-permissions';
	import { focusRoleControl } from '$lib/settings/person-focus';
	import EditProfileDialog from './EditProfileDialog.svelte';
	import PersonRow from './PersonRow.svelte';
	import PersonSheet from './PersonSheet.svelte';

	let {
		people,
		meId,
		actorRole,
		usage,
		sharing = {},
		setup = {},
		provider,
		mailEnabled = false,
		onrole,
		onshare,
		onreset,
		onsetuplink,
		onrevokelink,
		onunlink,
		ondelete,
		onprofile
	}: {
		people: PersonEntry[];
		meId: string;
		actorRole: UserRole;
		/** Palette counts for the profile dialog's picker. */
		usage: ColorUsage[];
		/** Who may share publicly, by id - loaded for admins only, so a
		 * member's view has no entries and no share controls. */
		sharing?: Record<string, boolean>;
		/** hasPassword, hasIdentity, the open setup link's expiry and the address with its state, by id - loaded for
		 * admins only, like `sharing`. */
		setup?: Record<
			string,
			{
				hasPassword: boolean;
				hasIdentity?: boolean;
				setupLinkExpiresAt?: string;
				email?: string;
				emailVerified?: boolean;
			}
		>;
		/** The identity provider's name; undefined while this instance has none. */
		provider?: string;
		/** Whether a changed address gets a confirmation mail. */
		mailEnabled?: boolean;
		/** Rejects when the API refused; the control then snaps back. */
		onrole: (person: PersonEntry, role: UserRole) => Promise<void>;
		/** Rejects when the API refused; the control then snaps back. */
		onshare: (person: PersonEntry, on: boolean) => Promise<void>;
		onreset: (person: PersonEntry) => void;
		/** Issues a fresh setup link and opens the dialog that shows it. */
		onsetuplink: (person: PersonEntry) => void;
		/** Revokes the open setup link. */
		onrevokelink: (person: PersonEntry) => void;
		/** Disconnects the person's identity-provider account. */
		onunlink: (person: PersonEntry) => void;
		ondelete: (person: PersonEntry) => void;
		/** The account as the profile dialog saved it. */
		onprofile: (person: PersonEntry | UserAccount) => void;
	} = $props();

	const uid = $props.id();

	// The order lives here, not in the API response: this is the one place
	// that decides what the reader sees, so a row added a second ago and the
	// same row after a reload land in the same spot - and a promoted person
	// moves under "Admins" the moment the change is saved.
	const groups = $derived(groupPeople(people));

	const titles: Record<UserRole, () => string> = {
		superadmin: m.users_role_owner,
		admin: m.users_group_admins,
		user: m.users_group_members
	};

	// The sheet and the profile dialog belong to the list, not to a row, and
	// find their person by id: a role change moves the row to another group,
	// which re-creates it, and a dialog owned by the row would vanish with it.
	// The ids stay set after closing, so the close transitions can play.
	let sheetOpen = $state(false);
	let sheetId = $state<string | null>(null);
	const sheetPerson = $derived(people.find((p) => p.id === sheetId));
	let editOpen = $state(false);
	let editId = $state<string | null>(null);
	const editPerson = $derived(people.find((p) => p.id === editId));
	const editPermissions = $derived(
		editPerson && rowPermissions(actorRole, editPerson, editPerson.id === meId)
	);

	// The component mounts with the id, and opens a tick later: mounted
	// already open, its first slide-in would not play.
	async function manage(person: PersonEntry) {
		sheetId = person.id;
		await tick();
		sheetOpen = true;
	}

	async function edit(person: PersonEntry) {
		editId = person.id;
		await tick();
		editOpen = true;
	}

	// A saved change moves the row to its new group, which re-creates it and
	// drops the focus the select had just returned to its trigger. The select
	// in the new row takes it back; the sheet, when it made the change, keeps
	// focus itself and stays open.
	async function changeRole(person: PersonEntry, role: UserRole) {
		await onrole(person, role);
		if (sheetOpen) return;
		await tick();
		focusRoleControl(person.id);
	}
</script>

<!--
	The people of the household as a book's list of contributors: one group
	per role, owner first, under an italic heading in the display face - the
	same voice the ingredient groups speak in - and a count beside every
	heading that can hold more than one. The heading names the role, so the
	rows do not repeat it.
-->
{#each groups as group (group.role)}
	{@const headingId = `${uid}-${group.role}`}
	<div class="mt-6 first:mt-2">
		<div class="flex items-baseline gap-2">
			<h3 id={headingId} class="font-display text-body-lg font-medium text-primary italic">
				{titles[group.role]()}
			</h3>
			{#if group.role !== 'superadmin'}
				<span class="text-caption text-text-muted tabular-nums">{group.people.length}</span>
			{/if}
		</div>
		<ul aria-labelledby={headingId} class="mt-1">
			{#each group.people as person (person.id)}
				<PersonRow
					{person}
					isSelf={person.id === meId}
					{actorRole}
					canShare={sharing[person.id]}
					hasPassword={setup[person.id]?.hasPassword}
					hasIdentity={setup[person.id]?.hasIdentity}
					{provider}
					setupLinkExpiresAt={setup[person.id]?.setupLinkExpiresAt}
					email={setup[person.id]?.email}
					emailVerified={setup[person.id]?.emailVerified}
					onrole={changeRole}
					{onshare}
					onmanage={manage}
					onedit={edit}
					{onreset}
					{onsetuplink}
					{onrevokelink}
					{onunlink}
					{ondelete}
				/>
			{/each}
		</ul>
	</div>
{/each}

{#if sheetPerson}
	<PersonSheet
		bind:open={sheetOpen}
		person={sheetPerson}
		{actorRole}
		isSelf={sheetPerson.id === meId}
		canShare={sharing[sheetPerson.id]}
		hasPassword={setup[sheetPerson.id]?.hasPassword}
		hasIdentity={setup[sheetPerson.id]?.hasIdentity}
		{provider}
		setupLinkExpiresAt={setup[sheetPerson.id]?.setupLinkExpiresAt}
		email={setup[sheetPerson.id]?.email}
		emailVerified={setup[sheetPerson.id]?.emailVerified}
		onrole={changeRole}
		{onshare}
		onedit={edit}
		{onreset}
		{onsetuplink}
		{onrevokelink}
		{onunlink}
		{ondelete}
	/>
{/if}
{#if editPerson && editPermissions}
	<EditProfileDialog
		bind:open={editOpen}
		user={editPerson}
		{usage}
		editProfile={editPermissions.editProfile}
		manageAccount={editPermissions.manageAccount}
		email={setup[editPerson.id]?.email}
		emailVerified={setup[editPerson.id]?.emailVerified}
		{mailEnabled}
		onsaved={onprofile}
	/>
{/if}
