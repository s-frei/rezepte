<script lang="ts">
	import type { PersonEntry, UserAccount, UserRole } from '#lib/api/users.js';
	import { m } from '#lib/paraglide/messages.js';
	import { groupPeople } from '#lib/settings/people-groups.js';
	import PersonTile from './PersonTile.svelte';

	let {
		people,
		meId,
		actorRole,
		setup = {},
		provider,
		motion = 0
	}: {
		people: PersonEntry[];
		meId: string;
		actorRole: UserRole;
		/** What an admin knows about each account, by id - loaded for admins
		 * only, so a member's view has no entries. */
		setup?: Record<string, UserAccount>;
		/** The identity provider's name; undefined while this instance has none. */
		provider?: string;
		/** How long an added row opens (`rowMotion`). */
		motion?: number;
	} = $props();

	const uid = $props.id();

	// The order lives here, not in the API response: this is the one place
	// that decides what the reader sees, so a row added a second ago and the
	// same row after a reload land in the same spot.
	const groups = $derived(groupPeople(people));

	const titles: Record<UserRole, () => string> = {
		superadmin: m.users_role_owner,
		admin: m.users_group_admins,
		user: m.users_group_members
	};
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
		<ul aria-labelledby={headingId} class="mt-2.5 flex flex-col gap-2.5">
			{#each group.people as person (person.id)}
				<PersonTile
					{person}
					isSelf={person.id === meId}
					{actorRole}
					hasPassword={setup[person.id]?.hasPassword}
					hasIdentity={setup[person.id]?.hasIdentity}
					{provider}
					setupLinkExpiresAt={setup[person.id]?.setupLinkExpiresAt}
					email={setup[person.id]?.email}
					emailVerified={setup[person.id]?.emailVerified}
					lastActiveAt={setup[person.id]?.lastActiveAt}
					{motion}
				/>
			{/each}
		</ul>
	</div>
{/each}
