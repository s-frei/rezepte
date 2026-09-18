<script lang="ts">
	import type { UserAccount, UserRole } from '$lib/api/users';
	import { m } from '$lib/paraglide/messages';
	import UserRow from './UserRow.svelte';

	let {
		users,
		meId,
		onrole,
		onreset,
		ondelete
	}: {
		users: UserAccount[];
		meId: string;
		onrole: (user: UserAccount, role: UserRole) => Promise<void>;
		onreset: (user: UserAccount) => void;
		ondelete: (user: UserAccount) => void;
	} = $props();

	const adminCount = $derived(users.filter((u) => u.role === 'admin').length);
</script>

<div
	aria-hidden="true"
	class="hidden grid-cols-[1fr_140px_240px] gap-3 border-b border-border pb-2 text-micro font-semibold tracking-[0.08em] text-text-muted uppercase md:grid"
>
	<span>{m.users_col_name()}</span>
	<span>{m.users_field_role()}</span>
	<span class="text-right">{m.users_col_actions()}</span>
</div>
<ul aria-label={m.settings_nav_users()}>
	{#each users as user (user.id)}
		<UserRow
			{user}
			isSelf={user.id === meId}
			isLastAdmin={user.role === 'admin' && adminCount === 1}
			{onrole}
			{onreset}
			{ondelete}
		/>
	{/each}
</ul>
