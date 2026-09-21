<script lang="ts">
	import Pencil from 'lucide-svelte/icons/pencil';
	import type { ColorUsage } from '$lib/api/auth';
	import type { UserAccount, UserRole } from '$lib/api/users';
	import Button from '$lib/components/ui/Button.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { m } from '$lib/paraglide/messages';
	import { roleLabel } from '$lib/roles';
	import { userColorClasses } from '$lib/user/color';
	import EditProfileDialog from './EditProfileDialog.svelte';

	let {
		user,
		isSelf,
		actorRole,
		usage,
		onrole,
		onreset,
		ondelete,
		onprofile
	}: {
		user: UserAccount;
		isSelf: boolean;
		/** The signed-in user's role; decides whether this row is manageable. */
		actorRole: UserRole;
		/** Palette counts for the profile dialog's picker. */
		usage: ColorUsage[];
		/** Rejects when the API refused; the select then snaps back. */
		onrole: (user: UserAccount, role: UserRole) => Promise<void>;
		onreset: (user: UserAccount) => void;
		ondelete: (user: UserAccount) => void;
		/** The account as the profile dialog saved it. */
		onprofile: (user: UserAccount) => void;
	} = $props();

	let editOpen = $state(false);

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
	// A name and a colour are the owner's to hand out, for every account but
	// their own: the profile page is the one place they rename themselves, so
	// nobody has to wonder which of two forms is the real one.
	const canEditProfile = $derived(!isOwner && !isSelf && actorRole === 'superadmin');

	const roleOptions = [
		{ value: 'admin', label: m.users_role_admin() },
		{ value: 'user', label: m.users_role_member() }
	];
	const initial = $derived(user.displayName.charAt(0).toUpperCase());
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
	The name takes the top line; role, the profile button and the two account
	actions share the next one and wrap only when they have to - "Passwort
	zurücksetzen" and "Löschen" together are 268px and a 360px card holds
	272px, so they take a line of their own and leave the role pill and the
	pencil on the one above, the pencil pushed to the right edge. Those three
	as one group would be 316px and would hang out over the card.

	`md:contents` dissolves the wrapper on desktop so all three become grid
	items; the pencil gets a track of its own, and the two that can be absent
	are placed by column rather than by order, so an owner's row - which has
	neither - still lines its actions up with everybody else's. The separator
	belongs between entries, so the last row drops it.
-->
<li
	class="grid grid-cols-[1fr_auto] items-center gap-3 border-b border-dashed border-border py-4 last:border-b-0 md:grid-cols-[1fr_140px_40px_280px]"
>
	<!-- The circle carries the person's own colour, the same one their recipe
	     cards are signed with, so the list reads as the cast of the household
	     rather than as eight rows of grey. `min-w-0` on the text block and
	     `shrink-0` on the circle are what keep a long name truncating instead
	     of squeezing the avatar into an ellipse. -->
	<div class="col-span-2 flex min-w-0 items-center gap-3 md:col-span-1">
		<span
			aria-hidden="true"
			class="flex size-9 shrink-0 items-center justify-center rounded-full initial-centred font-display font-semibold {userColorClasses(
				user.color
			)}"
		>
			{initial}
		</span>
		<div class="min-w-0">
			<p class="flex items-baseline gap-2">
				<span class="truncate text-body font-medium">{user.displayName}</span>
				{#if isSelf}
					<span class="shrink-0 text-micro text-text-muted">{m.users_you()}</span>
				{/if}
			</p>
			<!-- The login name stays on the row: it is what an admin types when
			     they help somebody sign in, and a display name may repeat. -->
			<p class="truncate text-micro text-text-muted">{user.username}</p>
		</div>
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

		<!-- An icon, where the two account actions are words: labelled, this
		     one would be a third pill on a row that already fills a phone, and
		     of the three it is the one a pencil says by itself. The name is
		     spelled out for anyone who cannot see it. -->
		{#if canEditProfile}
			<button
				type="button"
				aria-label={m.users_edit_profile()}
				title={m.users_edit_profile()}
				onclick={() => (editOpen = true)}
				class="inline-flex size-11 items-center justify-center rounded-full text-text-muted transition hover:bg-background hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:col-start-3 md:size-8"
			>
				<Pencil class="size-4" aria-hidden="true" />
			</button>
		{/if}

		<!-- 44px tall while a thumb is doing the tapping, back to the table's own
	     density on desktop. "Löschen" carries its own padded pill for the same
	     reason: bare text is a 50×20 target. The hints take the same cell as
	     the buttons they explain the absence of: as their own grid item they
	     would land in a second, implicit row, because the four columns are
	     already taken - the empty ones included. -->
		<div class="flex items-center gap-2 md:col-start-4 md:justify-end">
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
			{:else if isSelf}
				<!-- Before the owner's hint, because on the own row the useful
				     sentence is where to go, not what cannot be done here. -->
				<p class="text-micro text-text-muted md:text-right">{m.users_self_profile_hint()}</p>
			{:else if isOwner}
				<p class="text-micro text-text-muted md:text-right">{m.users_owner_hint()}</p>
			{/if}
		</div>
	</div>
</li>

<!-- One dialog per editable row rather than one for the whole table: it keeps
     its own copy of the form, so the close transition plays against the row it
     belongs to instead of against a target the page has already dropped. -->
{#if canEditProfile}
	<EditProfileDialog bind:open={editOpen} {user} {usage} onsaved={onprofile} />
{/if}
