<script lang="ts">
	import { DropdownMenu, Popover, Tooltip } from 'bits-ui';
	import { tick } from 'svelte';
	import Check from '@lucide/svelte/icons/check';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import KeyRound from '@lucide/svelte/icons/key-round';
	import MailCheck from '@lucide/svelte/icons/mail-check';
	import MailQuestionMark from '@lucide/svelte/icons/mail-question-mark';
	import Pencil from '@lucide/svelte/icons/pencil';
	import { resolve } from '$app/paths';
	import type { PersonEntry, UserRole } from '$lib/api/users';
	import Select from '$lib/components/ui/Select.svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/recipe/format';
	import { isAdminRole } from '$lib/roles';
	import {
		focusMenuTrigger,
		manageButtonId,
		menuTriggerId,
		personRowId
	} from '$lib/settings/person-focus';
	import { rowPermissions } from '$lib/settings/row-permissions';
	import PersonCard from '$lib/components/ui/PersonCard.svelte';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';

	let {
		person,
		isSelf,
		actorRole,
		canShare,
		hasPassword,
		hasIdentity,
		provider,
		setupLinkExpiresAt,
		email,
		emailVerified,
		onrole,
		onshare,
		onmanage,
		onedit,
		onreset,
		onsetuplink,
		onrevokelink,
		onunlink,
		ondelete
	}: {
		person: PersonEntry;
		isSelf: boolean;
		/** The signed-in user's role; decides which controls this row offers. */
		actorRole: UserRole;
		/** Whether this person may share publicly; undefined where the viewer
		 * cannot see it (a member's view of the list). */
		canShare?: boolean;
		/** Admin-only, like `canShare`: undefined where the viewer cannot see it. */
		hasPassword?: boolean;
		/** Admin-only: whether the person connected an identity provider. */
		hasIdentity?: boolean;
		/** The identity provider's name; undefined while this instance has none. */
		provider?: string;
		/** Admin-only; set while this person has an open setup link. */
		setupLinkExpiresAt?: string;
		/** Admin-only: the account's address; undefined where the viewer cannot see it. */
		email?: string;
		/** Admin-only: whether that address is confirmed. */
		emailVerified?: boolean;
		/** Rejects when the API refused; the select then snaps back. */
		onrole: (person: PersonEntry, role: UserRole) => Promise<void>;
		/** Rejects when the API refused; the menu item then snaps back. */
		onshare: (person: PersonEntry, on: boolean) => Promise<void>;
		/** A phone's tap on the row: opens the sheet with this person's actions. */
		onmanage: (person: PersonEntry) => void;
		onedit: (person: PersonEntry) => void;
		onreset: (person: PersonEntry) => void;
		/** Issues a fresh setup link and opens the dialog that shows it. */
		onsetuplink: (person: PersonEntry) => void;
		/** Revokes the open setup link. */
		onrevokelink: (person: PersonEntry) => void;
		/** Disconnects the person's identity-provider account. */
		onunlink: (person: PersonEntry) => void;
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
	// Only where the row already offers reset/delete: a member's view never
	// sees these fields at all (they are admin-only, like canShare).
	const setupStatus = $derived(
		!permissions.manageAccount
			? null
			: setupLinkExpiresAt
				? m.users_setup_link_open({ date: formatDate(setupLinkExpiresAt) })
				: !hasPassword && !hasIdentity
					? m.users_not_set_up()
					: null
	);
	// Its own line, not folded into setupStatus above: an account can carry
	// both at once (an open link on top of an already-connected identity), and
	// it reads as more prominent (text-text, not text-text-muted) than the
	// setup-link line - a standing fact about the account, not a transient one.
	const identityStatus = $derived(
		permissions.manageAccount && hasIdentity && provider
			? m.users_identity_status({ name: provider })
			: null
	);
	const canUnlink = $derived(permissions.manageAccount && hasIdentity && provider !== undefined);
	const hasActions = $derived(
		permissions.changeRole || permissions.manageAccount || permissions.editProfile || showShareMenu
	);
	// The wide screen's "..." trigger: everything about the account (reset,
	// setup link, revoke, disconnect, share, delete) lives in its menu now, so
	// it shows whenever that menu would hold at least one of them.
	const hasMenu = $derived(permissions.manageAccount || showShareMenu);
	const menuItemClass =
		'flex h-10 items-center rounded-sm px-3 text-body-sm text-text outline-none data-highlighted:bg-background';
	const destructiveMenuItemClass =
		'flex h-10 items-center rounded-sm px-3 text-body-sm text-destructive outline-none data-highlighted:bg-destructive-soft';

	const roleOptions = [
		{ value: 'admin', label: m.users_role_admin() },
		{ value: 'user', label: m.users_role_member() }
	];
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

	const emailLabel = $derived(
		emailVerified ? m.users_email_confirmed() : m.users_email_unconfirmed()
	);

	let menuOpen = $state(false);

	// Mirrors PersonSheet's `hand()`: closes the menu and moves focus to its
	// trigger by id before the action runs, so a dialog the action opens
	// (reset, setup link, delete) has a stable element to give focus back to
	// when it closes - not one still mid-way through the menu's own close.
	async function act(action: (person: PersonEntry) => void) {
		menuOpen = false;
		await tick();
		focusMenuTrigger(person.id);
		action(person);
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
	<!-- Phones: taps pass through to the row, which opens the sheet that
	     carries the card. From md up the circle is a button that opens it. -->
	<Popover.Root>
		<Popover.Trigger
			aria-label={m.person_card_open({ name: person.displayName })}
			class="pointer-events-none shrink-0 rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:pointer-events-auto"
		>
			<PersonMark {person} size="lg" />
		</Popover.Trigger>
		<Popover.Portal>
			<Popover.Content
				side="bottom"
				sideOffset={6}
				align="start"
				collisionPadding={12}
				class="z-50 max-w-80 rounded-2xl bg-surface p-4 text-text shadow-dialog"
			>
				<PersonCard {person} />
			</Popover.Content>
		</Popover.Portal>
	</Popover.Root>
	<div class="pointer-events-none min-w-0 flex-1 md:pointer-events-auto">
		<p class="flex items-baseline gap-2">
			<span class="truncate text-body font-medium">{person.displayName}</span>
			{#if isSelf}
				<span class="shrink-0 text-micro text-text-muted">{m.users_you()}</span>
			{/if}
		</p>
		<!-- The address's state in one symbol after the login name: what a
		     forgotten-password mail can reach. None without an address. The
		     owner's row carries it too. From md up the symbol is a focusable
		     tooltip trigger, so a keyboard reaches the words as well; on a
		     phone the whole row is one tap target, so the symbol stays inert
		     there and the sheet writes the state out. -->
		<p class="flex min-w-0 items-center gap-1 text-micro text-text-muted">
			<span class="truncate">{person.username}</span>
			{#if email}
				<Tooltip.Provider delayDuration={200}>
					<Tooltip.Root>
						<!-- Named by its label alone: the tooltip says the same words,
						     and as the trigger's description too a screen reader would
						     read them twice. -->
						<Tooltip.Trigger>
							{#snippet child({ props })}
								<button
									{...props}
									aria-describedby={undefined}
									aria-label={emailLabel}
									class="hidden shrink-0 rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:inline-flex"
								>
									{@render mailSymbol()}
								</button>
							{/snippet}
						</Tooltip.Trigger>
						<Tooltip.Portal>
							<Tooltip.Content
								side="right"
								sideOffset={6}
								class="z-50 rounded-md bg-inverse px-2.5 py-1.5 text-micro text-inverse-foreground shadow-dialog"
							>
								{emailLabel}
							</Tooltip.Content>
						</Tooltip.Portal>
					</Tooltip.Root>
				</Tooltip.Provider>
				<span class="inline-flex shrink-0 md:hidden">
					{@render mailSymbol()}
					<span class="sr-only">{emailLabel}</span>
				</span>
			{/if}
		</p>
		{#if identityStatus}
			<p class="flex items-center gap-1 text-micro text-text">
				<KeyRound class="size-3 shrink-0" aria-hidden="true" />
				<span class="truncate">{identityStatus}</span>
			</p>
		{/if}
		{#if setupStatus}
			<!-- Text only on every screen: "Revoke link" and "Disconnect …" are
			     the "..." menu's on a wide screen and the sheet's on a phone. -->
			<p class="text-micro text-text-muted">{setupStatus}</p>
		{/if}
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
		{#if permissions.editProfile || permissions.manageAccount}
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
		<!-- Everything else about the account - reset, setup link, revoke,
		     disconnect, share and delete - lives behind one "..." trigger:
		     alone each is reached rarely, and side by side they were wider
		     than the card. What's touched often (role, pencil) stays outside
		     it, above. -->
		{#if hasMenu}
			<DropdownMenu.Root bind:open={menuOpen}>
				<DropdownMenu.Trigger
					id={menuTriggerId(person.id)}
					aria-label={m.users_row_menu_aria({ username: person.username })}
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
						{#if permissions.manageAccount}
							<DropdownMenu.Item
								aria-label={m.users_reset_password_aria({ username: person.username })}
								onSelect={() => act(onreset)}
								class={menuItemClass}
							>
								{m.users_reset_password()}
							</DropdownMenu.Item>
							<DropdownMenu.Item
								aria-label={m.users_setup_link_action_aria({ username: person.username })}
								onSelect={() => act(onsetuplink)}
								class={menuItemClass}
							>
								{m.users_setup_link_action()}
							</DropdownMenu.Item>
							{#if setupLinkExpiresAt}
								<DropdownMenu.Item
									aria-label={m.users_setup_link_revoke_aria({ username: person.username })}
									onSelect={() => act(onrevokelink)}
									class={menuItemClass}
								>
									{m.users_setup_link_revoke()}
								</DropdownMenu.Item>
							{/if}
							{#if canUnlink}
								<DropdownMenu.Item
									aria-label={m.users_identity_unlink_aria({
										name: provider ?? '',
										username: person.username
									})}
									onSelect={() => act(onunlink)}
									class={menuItemClass}
								>
									{m.users_identity_unlink({ name: provider ?? '' })}
								</DropdownMenu.Item>
							{/if}
						{/if}
						{#if showShareMenu}
							{#if permissions.manageAccount}
								<DropdownMenu.Separator class="mx-1 my-1.5 h-px bg-border" />
							{/if}
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
						{/if}
						{#if permissions.manageAccount}
							<DropdownMenu.Separator class="mx-1 my-1.5 h-px bg-border" />
							<DropdownMenu.Item
								aria-label={m.users_delete_aria({ username: person.username })}
								onSelect={() => act(ondelete)}
								class={destructiveMenuItemClass}
							>
								{m.users_delete_account()}
							</DropdownMenu.Item>
						{/if}
					</DropdownMenu.Content>
				</DropdownMenu.Portal>
			</DropdownMenu.Root>
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

{#snippet mailSymbol()}
	{#if emailVerified}
		<MailCheck class="size-3 text-success-foreground" aria-hidden="true" />
	{:else}
		<MailQuestionMark class="size-3 text-text-muted" aria-hidden="true" />
	{/if}
{/snippet}
