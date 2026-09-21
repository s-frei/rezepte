<script lang="ts">
	import type { UserAccount, UserRole } from '$lib/api/users';
	import Button from '$lib/components/ui/Button.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { m } from '$lib/paraglide/messages';
	import { roleLabel } from '$lib/roles';

	let {
		user,
		isSelf,
		actorRole,
		onrole,
		onreset,
		ondelete
	}: {
		user: UserAccount;
		isSelf: boolean;
		/** The signed-in user's role; decides whether this row is manageable. */
		actorRole: UserRole;
		/** Rejects when the API refused; the select then snaps back. */
		onrole: (user: UserAccount, role: UserRole) => Promise<void>;
		onreset: (user: UserAccount) => void;
		ondelete: (user: UserAccount) => void;
	} = $props();

	// Writable derived (Svelte >= 5.25): follows `user.role` from the parent's
	// list, but can be overridden locally so a refused change snaps back even
	// though the parent's value never changed.
	let role = $derived<string>(user.role);

	// The instance owner: no role control and no actions at all, because the
	// API refuses every one of them. Absent rather than disabled - a control
	// that can never be used is noise, not information.
	const isOwner = $derived(user.role === 'superadmin');
	// Role changes belong to the owner alone, for every target; delete and
	// reset belong to any admin, but only over a plain member.
	const canChangeRole = $derived(!isOwner && actorRole === 'superadmin');
	const canManageAccount = $derived(
		!isOwner && !isSelf && (user.role === 'user' || actorRole === 'superadmin')
	);

	const roleOptions = [
		{ value: 'admin', label: m.users_role_admin() },
		{ value: 'user', label: m.users_role_member() }
	];
	const initial = $derived(user.username.charAt(0).toUpperCase());
	const pillClass = $derived(
		role === 'user' ? 'bg-background text-text-muted' : 'bg-accent text-accent-foreground'
	);

	async function changeRole(next: string) {
		try {
			await onrole(user, next as UserRole);
		} catch {
			role = user.role;
		}
	}
</script>

<!--
	The name takes the top line; role and actions share the next one and wrap
	only when they have to - a row with just "Löschen" (182px) stays on one
	line, one that also offers "Passwort zurücksetzen" (346px) breaks, since a
	360px card holds 302px. `md:contents` dissolves that wrapper on desktop so
	role and actions become the grid's own second and third column. The
	separator belongs between entries, so the last row drops it.
-->
<li
	class="grid grid-cols-[1fr_auto] items-center gap-3 border-b border-dashed border-border py-4 last:border-b-0 md:grid-cols-[1fr_140px_280px]"
>
	<div class="col-span-2 flex items-center gap-3 md:col-span-1">
		<span
			aria-hidden="true"
			class="flex size-8 shrink-0 items-center justify-center rounded-full bg-accent font-display font-semibold text-accent-foreground"
		>
			{initial}
		</span>
		<span class="text-body font-medium">{user.username}</span>
		{#if isSelf}
			<span class="text-micro text-text-muted">{m.users_you()}</span>
		{/if}
	</div>

	<div class="col-span-2 flex flex-wrap items-center justify-between gap-2 md:contents">
		<!-- One rule for the whole row: a role that cannot be changed is text,
		     a role that can is a control. -->
		{#if !canChangeRole}
			<span
				class="{pillClass} inline-flex h-11 items-center rounded-pill px-3 text-caption font-semibold md:h-8"
			>
				{roleLabel(user.role)}
			</span>
		{:else}
			<Select
				bind:value={role}
				options={roleOptions}
				label={m.users_role_aria({ username: user.username })}
				onchange={changeRole}
				variant="pill"
				class="{pillClass} h-11 md:h-8"
			/>
		{/if}

		<!-- 44px tall while a thumb is doing the tapping, back to the table's own
	     density on desktop. "Löschen" carries its own padded pill for the same
	     reason: bare text is a 50×20 target. The owner's hint takes the same
	     cell as the buttons it explains the absence of: as its own grid item
	     it would land in a fourth, implicit row, because the three columns
	     are already taken - the empty actions cell included. -->
		<div class="flex items-center gap-2 md:justify-end">
			<!-- Not for the own account: PATCH /users/:id ends every session of
		     the target, so resetting the own password would sign this admin
		     out. The profile page's password form is the way to do that. -->
			{#if canManageAccount}
				<Button
					variant="secondary"
					class="h-11 px-3 text-caption whitespace-nowrap md:h-8"
					label={m.users_reset_password_aria({ username: user.username })}
					onclick={() => onreset(user)}
				>
					{m.users_reset_password()}
				</Button>
				<button
					type="button"
					aria-label={m.users_delete_aria({ username: user.username })}
					onclick={() => ondelete(user)}
					class="inline-flex h-11 items-center rounded-pill px-3 text-caption font-semibold whitespace-nowrap text-destructive transition hover:bg-destructive-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:h-8"
				>
					{m.common_delete()}
				</button>
			{:else if isOwner}
				<p class="text-micro text-text-muted md:text-right">{m.users_owner_hint()}</p>
			{/if}
		</div>
	</div>
</li>
