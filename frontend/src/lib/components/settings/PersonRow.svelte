<script lang="ts">
	import { DropdownMenu } from 'bits-ui';
	import Check from '@lucide/svelte/icons/check';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import Pencil from '@lucide/svelte/icons/pencil';
	import { resolve } from '$app/paths';
	import type { PersonEntry, UserRole } from '$lib/api/users';
	import Button from '$lib/components/ui/Button.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { m } from '$lib/paraglide/messages';
	import { isAdminRole } from '$lib/roles';
	import { manageButtonId, personRowId } from '$lib/settings/person-focus';
	import { rowPermissions } from '$lib/settings/row-permissions';
	import { userColorClasses } from '$lib/user/color';

	let {
		person,
		isSelf,
		actorRole,
		canShare,
		onrole,
		onshare,
		onmanage,
		onedit,
		onreset,
		ondelete
	}: {
		person: PersonEntry;
		isSelf: boolean;
		/** The signed-in user's role; decides which controls this row offers. */
		actorRole: UserRole;
		/** Whether this person may share publicly; undefined where the viewer
		 * cannot see it (a member's view of the list). */
		canShare?: boolean;
		/** Rejects when the API refused; the select then snaps back. */
		onrole: (person: PersonEntry, role: UserRole) => Promise<void>;
		/** Rejects when the API refused; the menu item then snaps back. */
		onshare: (person: PersonEntry, on: boolean) => Promise<void>;
		/** A phone's tap on the row: opens the sheet with this person's actions. */
		onmanage: (person: PersonEntry) => void;
		onedit: (person: PersonEntry) => void;
		onreset: (person: PersonEntry) => void;
		ondelete: (person: PersonEntry) => void;
	} = $props();

	// Writable derived (Svelte >= 5.25): follows `person.role` from the list,
	// but can be overridden locally so a refused change snaps back even though
	// the list's value never changed.
	let role = $derived<string>(person.role);
	// Same trick for the share menu's checkbox: it flips at once, and snaps
	// back to `canShare` if the write is refused.
	let sharing = $derived<boolean>(canShare ?? false);

	const isOwner = $derived(person.role === 'superadmin');
	// Which controls this viewer gets on this row; rowPermissions holds the
	// rank rules and why each one is absent rather than disabled.
	const permissions = $derived(rowPermissions(actorRole, person, isSelf));
	const showShareMenu = $derived(permissions.toggleSharing && canShare !== undefined);
	const hasActions = $derived(
		permissions.changeRole || permissions.manageAccount || permissions.editProfile || showShareMenu
	);

	const roleOptions = [
		{ value: 'admin', label: m.users_role_admin() },
		{ value: 'user', label: m.users_role_member() }
	];
	const initial = $derived(person.displayName.charAt(0).toUpperCase());
	const pillClass = $derived(
		role === 'user' ? 'bg-background text-text-muted' : 'bg-accent text-accent-foreground'
	);

	async function toggleSharing(next: boolean) {
		try {
			await onshare(person, next);
		} catch {
			sharing = canShare ?? false;
		}
	}

	async function changeRole(next: string) {
		try {
			await onrole(person, next as UserRole);
		} catch {
			role = person.role;
		}
	}
</script>

<!--
	One line per person on every screen: the circle in their own color - the
	one their recipe cards are signed with - the display name, and the login
	name under it, which is what an admin types when helping somebody sign in
	and what tells two people called "Mia" apart. The group heading above
	names the role, so the row does not repeat it.

	A wide screen puts the row's actions beside it. A phone has no room for
	them, so the whole line is one tap target instead: it opens the sheet with
	this person's actions, or on the own row the profile page. That target is
	an overlay behind the text (`-z-10` inside the row's `isolate`), and the
	text lets taps through on a phone, so the pressed and focused states light
	up the line without covering the name. `-inset-x-3` lets the highlight
	reach past the avatar the way the mockups' rounded rows do.
-->
<li
	id={personRowId(person.id)}
	class="relative isolate flex min-h-14 items-center gap-3 py-2 md:border-b md:border-dashed md:border-border md:py-3 md:last:border-b-0"
>
	<span
		aria-hidden="true"
		class="pointer-events-none flex size-9 shrink-0 items-center justify-center rounded-full initial-centered font-display font-semibold md:pointer-events-auto {userColorClasses(
			person.color
		)}"
	>
		{initial}
	</span>
	<div class="pointer-events-none min-w-0 flex-1 md:pointer-events-auto">
		<p class="flex items-baseline gap-2">
			<span class="truncate text-body font-medium">{person.displayName}</span>
			{#if isSelf}
				<span class="shrink-0 text-micro text-text-muted">{m.users_you()}</span>
			{/if}
		</p>
		<p class="truncate text-micro text-text-muted">{person.username}</p>
	</div>

	<!-- Wide screens: the actions themselves, right-aligned, or a line saying
	     why there are none. A role that cannot be changed is not repeated
	     here - the group heading already says it. -->
	<div class="hidden shrink-0 items-center gap-2 md:flex">
		{#if permissions.changeRole}
			<!-- `data-role-control` is how the list finds this select again
			     once a role change has moved the row to another group. -->
			<span data-role-control class="contents">
				<Select
					bind:value={role}
					options={roleOptions}
					label={m.users_role_aria({ username: person.username })}
					onchange={changeRole}
					variant="pill"
					class="{pillClass} h-8"
				/>
			</span>
		{/if}
		{#if permissions.editProfile}
			<button
				type="button"
				aria-label={m.users_edit_profile()}
				title={m.users_edit_profile()}
				onclick={() => onedit(person)}
				class="inline-flex size-8 items-center justify-center rounded-full text-text-muted transition hover:bg-background hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				<Pencil class="size-4" aria-hidden="true" />
			</button>
		{/if}
		<!-- Whether this person may create public links: neither "manage the
		     account" nor "rename it", so a menu of its own beside the pencil,
		     with the one checkbox. -->
		{#if showShareMenu}
			<DropdownMenu.Root>
				<DropdownMenu.Trigger
					aria-label={m.users_share_menu()}
					title={m.users_share_menu()}
					class="inline-flex size-8 items-center justify-center rounded-full text-text-muted transition hover:bg-background hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<Ellipsis class="size-4" aria-hidden="true" />
				</DropdownMenu.Trigger>
				<DropdownMenu.Portal>
					<DropdownMenu.Content
						preventScroll={false}
						sideOffset={8}
						align="end"
						class="w-56 rounded-2xl border border-popover-border bg-popover p-2 shadow-dialog"
					>
						<DropdownMenu.CheckboxItem
							bind:checked={sharing}
							onCheckedChange={toggleSharing}
							class="flex h-10 items-center justify-between gap-2 rounded-sm px-3 text-body-sm text-text outline-none data-highlighted:bg-background"
						>
							{#snippet children({ checked })}
								<span>{m.users_share_toggle()}</span>
								{#if checked}
									<Check class="size-4 shrink-0 text-primary" aria-hidden="true" />
								{/if}
							{/snippet}
						</DropdownMenu.CheckboxItem>
					</DropdownMenu.Content>
				</DropdownMenu.Portal>
			</DropdownMenu.Root>
		{/if}
		{#if permissions.manageAccount}
			<Button
				variant="secondary"
				class="h-8 px-3 text-caption whitespace-nowrap"
				label={m.users_reset_password_aria({ username: person.username })}
				onclick={() => onreset(person)}
			>
				{m.users_reset_password()}
			</Button>
			<button
				type="button"
				aria-label={m.users_delete_aria({ username: person.username })}
				onclick={() => ondelete(person)}
				class="inline-flex h-8 items-center rounded-pill px-3 text-caption font-semibold whitespace-nowrap text-destructive transition hover:bg-destructive-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				{m.common_delete()}
			</button>
		{:else if isSelf}
			<!-- Before the owner's hint, because on the own row the useful
			     sentence is where to go, not what cannot be done here. -->
			<p class="text-micro text-text-muted">{m.users_self_profile_hint()}</p>
		{:else if isOwner && isAdminRole(actorRole)}
			<!-- Only where the other rows have actions: to a member, whose rows
			     have none, it would suggest the others can be removed. -->
			<p class="text-micro text-text-muted">{m.users_owner_hint()}</p>
		{/if}
	</div>

	<!-- Phones: the line itself is the control, marked by a chevron. -->
	{#if isSelf}
		<a
			href={resolve('/settings')}
			aria-label={m.users_open_own_profile()}
			class="absolute -inset-x-3 inset-y-0 -z-10 rounded-lg transition hover:bg-background focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary active:bg-background md:hidden"
		></a>
	{:else if hasActions}
		<button
			type="button"
			aria-haspopup="dialog"
			id={manageButtonId(person.id)}
			aria-label={m.users_manage_aria({ name: person.displayName, username: person.username })}
			onclick={() => onmanage(person)}
			class="absolute -inset-x-3 inset-y-0 -z-10 rounded-lg transition hover:bg-background focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary active:bg-background md:hidden"
		></button>
	{/if}
	{#if isSelf || hasActions}
		<ChevronRight
			class="pointer-events-none size-4 shrink-0 text-text-muted md:hidden"
			aria-hidden="true"
		/>
	{/if}
</li>
