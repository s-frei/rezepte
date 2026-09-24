<script lang="ts">
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import ChevronUp from '@lucide/svelte/icons/chevron-up';
	import type { ColorUsage } from '$lib/api/auth';
	import type { UserAccount, UserRole } from '$lib/api/users';
	import { m } from '$lib/paraglide/messages';
	import { DEFAULT_USER_SORT, nextSort, sortUsers } from '$lib/settings/user-sort';
	import UserRow from './UserRow.svelte';

	let {
		users,
		meId,
		actorRole,
		usage,
		onrole,
		onreset,
		ondelete,
		onprofile
	}: {
		users: UserAccount[];
		meId: string;
		actorRole: UserRole;
		/** Palette counts, handed straight to each row's profile dialog. */
		usage: ColorUsage[];
		onrole: (user: UserAccount, role: UserRole) => Promise<void>;
		onreset: (user: UserAccount) => void;
		ondelete: (user: UserAccount) => void;
		onprofile: (user: UserAccount) => void;
	} = $props();

	// The list's order lives here, not in the API response: this is the one
	// place that decides what the reader sees, so a row added a second ago and
	// the same row after a reload land in the same spot. A copy of the default,
	// because `$state` proxies the object it is handed and a shared module
	// constant would become shared state between two mounted tables.
	let sort = $state({ ...DEFAULT_USER_SORT });
	const sorted = $derived(sortUsers(users, sort));

	const headClass =
		'inline-flex items-center gap-1 rounded-sm uppercase transition hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
</script>

<!--
	Sorting is the desktop head's alone. Below `md` there is no head - the rows
	are stacked cards - so a phone keeps the default order by name rather than
	growing a second control for the same job.

	The state is part of each button's accessible name ("Rolle, aufsteigend
	sortiert") rather than an `aria-sort`: that attribute is defined on
	`columnheader` only, and a real `columnheader` needs a `row` inside a
	`table`, which this grid of `<li>` cards deliberately is not. Without it
	the chevron would be the only carrier of the state, and it is decoration.
	`uppercase` is repeated on the buttons because a button does not inherit
	`text-transform`; the `title` is the mouse user's hint that the head sorts.
-->
<div
	class="hidden grid-cols-[1fr_140px_40px_280px] gap-3 border-b border-border pb-2 text-micro font-semibold tracking-[0.08em] text-text-muted uppercase md:grid"
>
	<span>
		<button
			type="button"
			class={headClass}
			title={m.users_sort_by_name()}
			onclick={() => (sort = nextSort(sort, 'name'))}
		>
			{m.users_col_name()}
			{#if sort.column === 'name'}
				<span class="sr-only">
					{sort.direction === 'asc' ? m.users_sort_ascending() : m.users_sort_descending()}
				</span>
				{#if sort.direction === 'asc'}
					<ChevronUp class="size-3" aria-hidden="true" />
				{:else}
					<ChevronDown class="size-3" aria-hidden="true" />
				{/if}
			{/if}
		</button>
	</span>
	<span>
		<button
			type="button"
			class={headClass}
			title={m.users_sort_by_role()}
			onclick={() => (sort = nextSort(sort, 'role'))}
		>
			{m.users_field_role()}
			{#if sort.column === 'role'}
				<span class="sr-only">
					{sort.direction === 'asc' ? m.users_sort_ascending() : m.users_sort_descending()}
				</span>
				{#if sort.direction === 'asc'}
					<ChevronUp class="size-3" aria-hidden="true" />
				{:else}
					<ChevronDown class="size-3" aria-hidden="true" />
				{/if}
			{/if}
		</button>
	</span>
	<!-- Column 3 belongs to the rows' pencil, and a pencil needs no heading:
	     the label goes over the two buttons in column 4. -->
	<span class="col-start-4 text-right">{m.users_col_actions()}</span>
</div>
<ul aria-label={m.settings_nav_users()}>
	{#each sorted as user (user.id)}
		<UserRow
			{user}
			isSelf={user.id === meId}
			{actorRole}
			{usage}
			{onrole}
			{onreset}
			{ondelete}
			{onprofile}
		/>
	{/each}
</ul>
