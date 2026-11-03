<script lang="ts">
	import { DropdownMenu, Popover, Tooltip } from 'bits-ui';
	import { tick } from 'svelte';
	import { cubicOut } from 'svelte/easing';
	import { MediaQuery } from 'svelte/reactivity';
	import { slide } from 'svelte/transition';
	import BookOpen from '@lucide/svelte/icons/book-open';
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import Crown from '@lucide/svelte/icons/crown';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import KeyRound from '@lucide/svelte/icons/key-round';
	import Link from '@lucide/svelte/icons/link';
	import MailCheck from '@lucide/svelte/icons/mail-check';
	import MailQuestionMark from '@lucide/svelte/icons/mail-question-mark';
	import Pencil from '@lucide/svelte/icons/pencil';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import { resolve } from '$app/paths';
	import type { PersonEntry, UserRole } from '#lib/api/users.js';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Select from '#lib/components/ui/Select.svelte';
	import PersonCard from '#lib/components/ui/PersonCard.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { m } from '#lib/paraglide/messages.js';
	import { formatDate } from '#lib/recipe/format.js';
	import { isAdminRole } from '#lib/roles.js';
	import { focusMenuTrigger, menuTriggerId, personRowId } from '#lib/settings/person-focus.js';
	import { rowPermissions } from '#lib/settings/row-permissions.js';

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
		createdAt,
		lastRecipeTitle,
		onrole,
		onshare,
		onedit,
		onreset,
		onsetuplink,
		onrevokelink,
		onunlink,
		ondelete,
		/** How long the card opens or closes; 0 unless the viewer just added or removed it (`rowMotion`). */
		motion = 0
	}: {
		person: PersonEntry;
		isSelf: boolean;
		/** The signed-in user's role; decides which controls this card offers. */
		actorRole: UserRole;
		/** Admin-only, like every field down to `lastRecipeTitle`: undefined
		 * where the viewer cannot see it (a member's view of the list). */
		canShare?: boolean;
		hasPassword?: boolean;
		hasIdentity?: boolean;
		/** The identity provider's name; undefined while this instance has none. */
		provider?: string;
		/** Set while this person has an open setup link. */
		setupLinkExpiresAt?: string;
		email?: string;
		emailVerified?: boolean;
		createdAt?: string;
		lastRecipeTitle?: string | null;
		/** Rejects when the API refused; the select then snaps back. */
		onrole: (person: PersonEntry, role: UserRole) => Promise<void>;
		/** Rejects when the API refused; the menu item then snaps back. */
		onshare: (person: PersonEntry, on: boolean) => Promise<void>;
		onedit: (person: PersonEntry) => void;
		onreset: (person: PersonEntry) => void;
		/** Issues a fresh setup link and opens the dialog that shows it. */
		onsetuplink: (person: PersonEntry) => void;
		/** Revokes the open setup link. */
		onrevokelink: (person: PersonEntry) => void;
		/** Disconnects the person's identity-provider account. */
		onunlink: (person: PersonEntry) => void;
		ondelete: (person: PersonEntry) => void;
		motion?: number;
	} = $props();

	// Writable derived (Svelte >= 5.25): follows `person.role` from the list,
	// but can be overridden locally so a refused change snaps back even though
	// the list's value never changed.
	let role = $derived<string>(person.role);
	// Same trick for the share menu's checkbox.
	let sharing = $derived<boolean>(canShare ?? false);
	let open = $state(false);
	let cardOpen = $state(false);
	// The person card is a bottom sheet below `md`, as everywhere else.
	const phone = new MediaQuery('max-width: 767.98px');

	// Everyone sees the card; only an admin gets its controls and details.
	const isAdmin = $derived(isAdminRole(actorRole));
	const isOwner = $derived(person.role === 'superadmin');
	// Which controls this viewer gets on this card; rowPermissions holds the
	// rank rules and why each one is absent rather than disabled.
	const permissions = $derived(rowPermissions(actorRole, person, isSelf));
	const showShareMenu = $derived(permissions.toggleSharing && canShare !== undefined);
	const setupStatus = $derived(
		!permissions.manageAccount
			? null
			: setupLinkExpiresAt
				? m.users_setup_link_open({ date: formatDate(setupLinkExpiresAt) })
				: !hasPassword && !hasIdentity
					? m.users_not_set_up()
					: null
	);
	// Its own line: an account can carry both at once (an open link on top of
	// an already-connected identity), and it is a standing fact, not a
	// transient one.
	const identityStatus = $derived(
		permissions.manageAccount && hasIdentity && provider
			? m.users_identity_status({ name: provider })
			: null
	);
	const canUnlink = $derived(permissions.manageAccount && hasIdentity && provider !== undefined);
	const canEdit = $derived(permissions.editProfile || permissions.manageAccount);
	// Everything about the account (reset, setup link, revoke, disconnect,
	// share, delete) lives in the "..." menu.
	const hasMenu = $derived(permissions.manageAccount || showShareMenu);
	// The own card's pencil leads to the profile page, where anyone renames
	// themselves - a member's own card included, its only control.
	const hasControls = $derived(
		isSelf || (isAdmin && (isOwner || permissions.changeRole || canEdit || hasMenu))
	);

	const lists = $derived(person.listCount ?? 0);
	const recipes = $derived(person.recipeCount ?? 0);
	const listsText = $derived(
		lists === 0
			? m.users_lists_none()
			: lists === 1
				? m.users_lists_count_one()
				: m.users_lists_count({ count: lists })
	);
	const recipesText = $derived(
		recipes === 0
			? m.users_recipes_none()
			: recipes === 1
				? m.users_recipes_count_one()
				: m.users_recipes_count({ count: recipes })
	);
	const joined = $derived.by(() => {
		const date = createdAt ? new Date(createdAt) : null;
		return date && !Number.isNaN(date.getTime())
			? date.toLocaleDateString(getLocale(), { month: 'long', year: 'numeric' })
			: '';
	});
	const signIn = $derived(
		[hasPassword ? m.users_detail_signin_password() : '', hasIdentity ? (provider ?? '') : '']
			.filter(Boolean)
			.join(' · ') || m.users_not_set_up()
	);

	const menuItemClass =
		'flex h-10 items-center rounded-sm px-3 text-body-sm text-text outline-none data-highlighted:bg-background';
	const destructiveMenuItemClass =
		'flex h-10 items-center rounded-sm px-3 text-body-sm text-destructive outline-none data-highlighted:bg-destructive-soft';
	const iconButtonClass =
		'inline-flex size-10 items-center justify-center rounded-full text-text-muted transition hover:bg-background hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:size-8';

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

	// Closes the menu and moves focus to its trigger by id before the action
	// runs, so a dialog the action opens has a stable element to give focus
	// back to when it closes.
	async function act(action: (person: PersonEntry) => void) {
		menuOpen = false;
		await tick();
		focusMenuTrigger(person.id);
		action(person);
	}

	// The number in bold, the words around it as the catalog has them.
	function split(text: string, count: number): [string, string, string] {
		const at = count === 0 ? -1 : text.indexOf(String(count));
		if (at < 0) return [text, '', ''];
		return [text.slice(0, at), String(count), text.slice(at + String(count).length)];
	}
</script>

<!--
	One card per person: the circle in their own color, the display name and
	the login name with the address's state - what tells two people called
	"Mia" apart - and under a dashed rule how many shopping lists and recipes
	they have. An admin gets the controls beside those counts and a chevron
	that swaps them for the account's details; the controls stay put.
-->
<li
	transition:slide={{ duration: motion, easing: cubicOut }}
	id={personRowId(person.id)}
	class="rounded-xl border border-border p-3.5 md:px-4"
>
	<div class="flex items-center gap-2.5 md:gap-3">
		{#if phone.current}
			<button
				type="button"
				aria-label={m.person_card_open({ name: person.displayName })}
				onclick={() => (cardOpen = true)}
				class="shrink-0 rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				<PersonMark {person} size="lg" class="md:size-11" />
			</button>
			<BottomSheet bind:open={cardOpen} closeLabel={m.common_close()}>
				<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">
					<PersonCard {person} asTitle onnavigate={() => (cardOpen = false)} />
				</div>
			</BottomSheet>
		{:else}
			<Popover.Root>
				<Popover.Trigger
					aria-label={m.person_card_open({ name: person.displayName })}
					class="shrink-0 rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<PersonMark {person} size="lg" class="md:size-11" />
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
		{/if}
		<div class="min-w-0 flex-1">
			<p class="flex items-baseline gap-2">
				<span class="min-w-0 text-body font-medium wrap-break-word md:truncate">
					{person.displayName}
				</span>
				{#if isSelf}
					<span class="shrink-0 text-micro text-text-muted">{m.users_you()}</span>
				{/if}
			</p>
			<!-- The address's state in one symbol after the login name: what a
			     forgotten-password mail can reach. None without an address. -->
			<p class="flex min-w-0 items-center gap-1 text-micro text-text-muted">
				<span class="truncate">{person.username}</span>
				{#if email}
					<Tooltip.Provider delayDuration={200}>
						<Tooltip.Root>
							<!-- Named by its label alone: as the trigger's description too
							     a screen reader would read the words twice. -->
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
					<!-- A tooltip does not open on touch: a phone gets the words for
					     a screen reader, and the details spell the state out. -->
					<span class="inline-flex shrink-0 md:hidden">
						{@render mailSymbol()}
						<span class="sr-only">{emailLabel}</span>
					</span>
				{/if}
			</p>
			{#if identityStatus}
				<p class="flex items-center gap-1 text-micro text-text">
					<KeyRound
						class="mt-0.5 size-3 shrink-0 self-start md:mt-0 md:self-center"
						aria-hidden="true"
					/>
					<span class="min-w-0 md:truncate">{identityStatus}</span>
				</p>
			{/if}
		</div>
		{#if isAdmin}
			<button
				type="button"
				aria-expanded={open}
				aria-controls={open ? `${personRowId(person.id)}-details` : undefined}
				aria-label={open
					? m.users_details_hide({ name: person.displayName })
					: m.users_details_show({ name: person.displayName })}
				onclick={() => (open = !open)}
				class="inline-flex size-10 shrink-0 items-center justify-center self-start rounded-full text-text-muted transition hover:bg-background hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-expanded:bg-background aria-expanded:text-text"
			>
				<ChevronDown
					class="size-[18px] transition-transform motion-reduce:transition-none {open
						? 'rotate-180'
						: ''}"
					aria-hidden="true"
				/>
			</button>
		{/if}
	</div>

	{#if setupStatus}
		<!-- Text only: "Revoke link" lives in the "..." menu. -->
		<p
			class="mt-2.5 flex items-start gap-1.5 rounded-md border border-dashed border-border px-3 py-2 text-micro text-text-muted"
		>
			<Link class="mt-0.5 size-3 shrink-0" aria-hidden="true" />
			<span>{setupStatus}</span>
		</p>
	{/if}

	{#if open}
		<dl
			id="{personRowId(person.id)}-details"
			class="mt-3 grid grid-cols-1 rounded-lg bg-background px-3.5 py-1 md:grid-cols-3 md:gap-x-5 md:gap-y-3.5 md:px-4 md:py-3.5"
		>
			{@render detail(m.users_detail_lists(), listsText)}
			{@render detail(
				m.users_detail_recipes(),
				recipesText,
				recipes > 0 && lastRecipeTitle ? m.users_detail_last_recipe({ title: lastRecipeTitle }) : ''
			)}
			{@render detail(
				m.users_detail_email(),
				email || m.users_detail_email_none(),
				email ? emailLabel : '',
				true
			)}
			{@render detail(m.users_detail_joined(), joined)}
			{@render detail(m.users_detail_signin(), signIn)}
			{@render detail(
				m.users_detail_public_sharing(),
				canShare ? m.users_detail_sharing_allowed() : m.users_detail_sharing_not_allowed()
			)}
		</dl>
	{/if}

	{#if !open || hasControls}
		<div
			class="mt-3 flex flex-wrap items-center justify-between gap-x-4 gap-y-2.5 border-t border-dashed border-border pt-3"
		>
			{#if !open}
				<!-- One fact per line on a phone, where side by side they wrapped
				     differently from card to card. -->
				<p class="flex flex-col items-start gap-x-4.5 gap-y-1.5 md:flex-row md:flex-wrap">
					{@render fact(ShoppingBasket, lists, listsText)}
					{@render fact(BookOpen, recipes, recipesText)}
				</p>
			{/if}
			{#if hasControls}
				<div class="flex w-full items-center gap-1 md:ml-auto md:w-auto">
					<span class="mr-auto flex md:mr-0">
						{#if isOwner}
							<span
								class="inline-flex h-8 items-center gap-1.5 rounded-pill bg-accent px-3 text-body-sm font-semibold whitespace-nowrap text-accent-foreground"
							>
								<Crown class="size-3.5" aria-hidden="true" />
								{m.users_role_owner()}
							</span>
						{:else if permissions.changeRole}
							<!-- `data-role-control` is how the list finds this select again
							     once a role change has moved the card to another group. -->
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
					</span>
					{#if isSelf}
						<a
							href={resolve('/settings')}
							aria-label={m.users_open_own_profile()}
							title={m.users_open_own_profile()}
							class={iconButtonClass}
						>
							<Pencil class="size-4" aria-hidden="true" />
						</a>
					{:else if canEdit}
						<button
							type="button"
							aria-label={m.users_edit_profile()}
							title={m.users_edit_profile()}
							onclick={() => onedit(person)}
							class={iconButtonClass}
						>
							<Pencil class="size-4" aria-hidden="true" />
						</button>
					{/if}
					{#if hasMenu}
						<DropdownMenu.Root bind:open={menuOpen}>
							<DropdownMenu.Trigger
								id={menuTriggerId(person.id)}
								aria-label={m.users_row_menu_aria({ username: person.username })}
								title={m.users_share_menu()}
								class={iconButtonClass}
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
					{:else if isSelf && isAdmin}
						<!-- Keeps the own pencil under the others'; a member has no others. -->
						<span class="size-10 md:size-8" aria-hidden="true"></span>
					{/if}
				</div>
			{/if}
		</div>
	{/if}
</li>

{#snippet fact(Icon: typeof BookOpen, count: number, text: string)}
	{@const [before, number, after] = split(text, count)}
	<span
		class="inline-flex items-center gap-1.5 text-body-sm whitespace-nowrap {count
			? 'text-text'
			: 'text-text-muted'}"
	>
		<Icon class="size-3.5 shrink-0 text-text-muted" aria-hidden="true" />
		<span>{before}<b class="font-semibold">{number}</b>{after}</span>
	</span>
{/snippet}

{#snippet detail(label: string, value: string, note = '', mail = false)}
	<div class="border-b border-dashed border-border py-2.5 last:border-b-0 md:border-b-0 md:py-0">
		<dt class="text-caption text-text-muted md:text-label md:font-semibold md:uppercase">
			{label}
		</dt>
		<dd class="mt-0.5 text-body-sm wrap-anywhere text-text md:mt-1">
			{value}
			{#if note}
				<small class="mt-0.5 flex items-center gap-1 text-micro text-text-muted">
					{#if mail}{@render mailSymbol()}{/if}
					{note}
				</small>
			{/if}
		</dd>
	</div>
{/snippet}

{#snippet mailSymbol()}
	{#if emailVerified}
		<MailCheck class="size-3 shrink-0 text-success-foreground" aria-hidden="true" />
	{:else}
		<MailQuestionMark class="size-3 shrink-0 text-text-muted" aria-hidden="true" />
	{/if}
{/snippet}
