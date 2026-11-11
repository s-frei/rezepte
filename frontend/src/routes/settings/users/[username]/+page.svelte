<script lang="ts">
	import { DropdownMenu } from 'bits-ui';
	import { tick, untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Activity from '@lucide/svelte/icons/activity';
	import BookOpen from '@lucide/svelte/icons/book-open';
	import Calendar from '@lucide/svelte/icons/calendar';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import Globe from '@lucide/svelte/icons/globe';
	import KeyRound from '@lucide/svelte/icons/key-round';
	import Link from '@lucide/svelte/icons/link';
	import Shield from '@lucide/svelte/icons/shield';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import Unlink from '@lucide/svelte/icons/unlink';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import { getOidc } from '#lib/api/oidc.js';
	import { getSettings } from '#lib/api/settings.js';
	import {
		deleteUser,
		issueSetupLink,
		listPeople,
		listUsers,
		removeUserAvatar,
		resendUserEmailConfirmation,
		revokeSetupLink,
		setUserAvatar,
		unlinkUserIdentity,
		updateUser,
		type PersonEntry,
		type SetupLinkInfo,
		type UserAccount,
		type UserRole
	} from '#lib/api/users.js';
	import ColorPicker from '#lib/components/settings/ColorPicker.svelte';
	import EmailDialog from '#lib/components/settings/EmailDialog.svelte';
	import EmailRow from '#lib/components/settings/EmailRow.svelte';
	import ProfileHead from '#lib/components/settings/ProfileHead.svelte';
	import ResetPasswordDialog from '#lib/components/settings/ResetPasswordDialog.svelte';
	import SendSetupLinkDialog from '#lib/components/settings/SendSetupLinkDialog.svelte';
	import SettingsGroup from '#lib/components/settings/SettingsGroup.svelte';
	import SettingsLayout from '#lib/components/settings/SettingsLayout.svelte';
	import SettingsRow from '#lib/components/settings/SettingsRow.svelte';
	import SetupLinkDialog from '#lib/components/settings/SetupLinkDialog.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import ConfirmDialog from '#lib/components/ui/ConfirmDialog.svelte';
	import EmptyState from '#lib/components/ui/EmptyState.svelte';
	import Select from '#lib/components/ui/Select.svelte';
	import Skeleton from '#lib/components/ui/Skeleton.svelte';
	import Switch from '#lib/components/ui/Switch.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { optimistic } from '#lib/optimistic.js';
	import { formatDate } from '#lib/recipe/format.js';
	import { isAdminRole, roleLabel } from '#lib/roles.js';
	import { lastActive } from '#lib/settings/last-active.js';
	import { listsText, personPageTarget, recipesText } from '#lib/settings/person-page.js';
	import type { UserColor } from '#lib/user/color.js';
	import { userColorLabel } from '#lib/user/color-label.js';
	import { firstWord } from '#lib/user/holders.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const uid = $props.id();

	let people = $state<PersonEntry[]>([]);
	let loading = $state(true);
	let loadFailed = $state(false);
	let emailOpen = $state(false);

	// What only an admin may read about each account, by id, from
	// `listUsers()`; empty for anyone else.
	let accounts = $state<Record<string, UserAccount>>({});
	/** The provider's name while this instance has one; unlinking needs it. */
	let provider = $state<string | undefined>(undefined);
	/** Whether Rezepte sends mail: a setup link then asks where to first. */
	let mailEnabled = $state(false);

	// The dialogs, mounted while the page is; each acts on the person shown.
	let resetOpen = $state(false);
	let deleteOpen = $state(false);
	let unlinkOpen = $state(false);
	let sendOpen = $state(false);
	let setupLinkOpen = $state(false);
	let setupLinkInfo = $state<SetupLinkInfo | null>(null);

	const person = $derived(people.find((p) => p.username === data.username));
	const verdict = $derived(
		personPageTarget({ id: data.viewer?.id ?? '', role: data.viewer?.role ?? 'user' }, person)
	);
	const permissions = $derived(verdict.kind === 'show' ? verdict.permissions : null);

	// The people decide whether this page shows at all; what only an admin may
	// read, and the extras the rows need, come with them. A viewer who is no
	// admin never stays here, so they never ask for the admin's part.
	//
	// `loadSeq` names the newest load: an answer for a person the viewer has
	// since left, or for a page already gone, neither lands nor redirects.
	let loadSeq = 0;
	async function load(quiet = false) {
		const seq = ++loadSeq;
		if (!quiet) loading = true;
		loadFailed = false;
		const admin = isAdminRole(data.viewer?.role);
		try {
			const [list, users, oidc, settings] = await Promise.all([
				listPeople(),
				admin ? listUsers() : Promise.resolve([]),
				admin ? getOidc().catch(() => null) : null,
				admin ? getSettings().catch(() => null) : null
			]);
			if (seq !== loadSeq) return;
			people = list;
			accounts = Object.fromEntries(users.map((a) => [a.id, a]));
			provider = oidc?.enabled ? oidc.name : undefined;
			mailEnabled = settings?.mailEnabled ?? false;
			if (verdict.kind === 'redirect') {
				await goto(resolve(verdict.to), { replaceState: true });
			}
		} catch (error) {
			if (seq !== loadSeq) return;
			// On a 401 the client is already navigating to the login page.
			loadFailed = !isSignedOut(error);
		}
		loading = false;
	}

	// Again for every person: going from one page to the next keeps this
	// component and changes only the username. The cleanup retires the load
	// in flight, on the next person and on leaving alike.
	$effect(() => {
		void data.username;
		untrack(() => void load());
		return () => loadSeq++;
	});

	// Merged, not swapped: a PATCH answer carries no list or recipe counts.
	function replace(updated: PersonEntry) {
		people = people.map((p) => (p.id === updated.id ? { ...p, ...updated } : p));
	}

	// Only the fields a write is known to change: a PATCH answer carries no
	// identity or activity.
	function patchAccount(id: string, patch: Partial<UserAccount>) {
		if (accounts[id]) Object.assign(accounts[id], patch);
	}

	const setup = $derived(person ? accounts[person.id] : undefined);
	const firstName = $derived(person ? firstWord(person.displayName) : '');

	// Writable deriveds: they follow the person, and snap back when a change
	// is refused although the person never changed.
	let color = $derived((person?.color ?? 'amber') as UserColor);
	let role = $derived<string>(person?.role ?? 'user');
	let sharing = $derived(setup?.canSharePublicly ?? false);

	const roleOptions = [
		{ value: 'admin', label: m.users_role_admin() },
		{ value: 'user', label: m.users_role_member() }
	];

	// How the person gets in: an open link first, since it is what the admin
	// came to manage, then what the account has.
	const signIn = $derived.by(() => {
		if (!setup) return '';
		if (setup.setupLinkExpiresAt) {
			return m.users_setup_link_open({ date: formatDate(setup.setupLinkExpiresAt) });
		}
		const identity = setup.hasIdentity ? provider : undefined;
		if (setup.hasPassword && identity) return m.person_signin_both({ name: identity });
		if (identity) return m.users_identity_status({ name: identity });
		if (setup.hasPassword) return m.person_signin_password();
		return m.users_not_set_up();
	});

	// What the Overview group says: the month the account was made, and how
	// long ago it was last active.
	const joined = $derived.by(() => {
		const created = setup?.createdAt ? new Date(setup.createdAt) : null;
		return created && !Number.isNaN(created.getTime())
			? created.toLocaleDateString(getLocale(), { month: 'long', year: 'numeric' })
			: '';
	});
	const activity = $derived(setup ? lastActive(setup.lastActiveAt) : null);

	/** The toast for a refused write on someone else's account. */
	function refused(error: unknown) {
		if (error instanceof ApiError && error.status === 409) {
			toast.error(m.users_owner_protected());
		} else if (error instanceof ApiError && error.status === 403) {
			toast.error(m.users_rank_required());
		} else if (!isSignedOut(error)) {
			toast.error(m.users_update_error());
		}
	}

	/**
	 * A profile edit saved (a name, a color, an address). A PATCH answer
	 * carries the address, and a changed one is unconfirmed: the row follows
	 * at once.
	 */
	function profileSaved(updated: UserAccount) {
		replace(updated);
		patchAccount(updated.id, {
			email: updated.email,
			emailVerified: updated.emailVerified,
			emailConfirmation: updated.emailConfirmation,
			emailSetupLink: updated.emailSetupLink
		});
	}

	// Quick picks and flips end on the value the server holds (see
	// `optimistic`); every answer it confirms is stored, and only the newest
	// one toasts. A request for someone the page no longer shows, such as an
	// Undo from an earlier page, leaves the controls alone.
	let roleOf = '';
	const sendRole = optimistic(
		() => role,
		(value) => {
			if (person?.id === roleOf) role = value;
		}
	);
	let sharingOf = '';
	const sendSharing = optimistic(
		() => sharing,
		(value) => {
			if (person?.id === sharingOf) sharing = value;
		}
	);

	/**
	 * A role takes effect with one tap and carries rights, so the toast names
	 * the new role and offers the way back: a stray tap costs a second tap,
	 * not a trip into the sheet to find the old role again.
	 */
	async function saveRole(user: PersonEntry, next: UserRole) {
		const previous = user.role;
		roleOf = user.id;
		try {
			const { answer: updated, latest } = await sendRole(
				next,
				() => updateUser(user.id, { role: next }),
				(u) => {
					replace(u);
					return u.role;
				}
			);
			if (!latest) return;
			toast.success(m.users_role_changed({ name: updated.displayName, role: roleLabel(next) }), {
				action: { label: m.common_undo(), onClick: () => void saveRole(updated, previous) }
			});
		} catch (error) {
			refused(error);
		}
	}

	function changeRole(next: string) {
		if (person) void saveRole(person, next as UserRole);
	}

	let colorTimer: ReturnType<typeof setTimeout> | undefined;
	let colorSeq = 0;
	let colorFrom: UserColor | null = null;

	/**
	 * A color applies at once like a role, so it gets the same toast with the
	 * way back. The picker is a radio group whose arrow keys move the
	 * selection, so a run across the palette is debounced into one save of
	 * the color it lands on, and the toast's Undo returns to the color from
	 * before the run. An answer overtaken by a newer pick is dropped, toast
	 * and all. Rejects when the run's save fails, already toasted, so the
	 * control can snap back; an overtaken pick never settles.
	 */
	function setColor(user: PersonEntry, next: UserColor): Promise<void> {
		clearTimeout(colorTimer);
		colorFrom ??= user.color as UserColor;
		return new Promise((resolve, reject) => {
			colorTimer = setTimeout(() => {
				// Numbered when the save starts, not at the pick: an Undo sent
				// while this pick waits must not outrank it.
				const seq = ++colorSeq;
				const previous = colorFrom ?? (user.color as UserColor);
				colorFrom = null;
				saveColor(user, next, previous, seq).then(resolve, reject);
			}, 400);
		});
	}

	async function saveColor(user: PersonEntry, next: UserColor, previous: UserColor, seq: number) {
		try {
			const updated = await updateUser(user.id, { color: next });
			if (seq !== colorSeq) return;
			profileSaved(updated);
			toast.success(
				m.users_color_changed({ name: updated.displayName, color: userColorLabel(next) }),
				{
					action: {
						label: m.common_undo(),
						onClick: () => void saveColor(updated, previous, next, ++colorSeq).catch(() => {})
					}
				}
			);
		} catch (error) {
			if (seq !== colorSeq) return;
			refused(error);
			throw error;
		}
	}

	async function changeColor(next: UserColor) {
		if (!person) return;
		try {
			await setColor(person, next);
		} catch {
			color = person.color;
		}
	}

	async function changeSharing(next: boolean) {
		if (!person) return;
		const { id, username } = person;
		sharingOf = id;
		try {
			const { latest } = await sendSharing(
				next,
				() => updateUser(id, { canSharePublicly: next }),
				(u) => {
					patchAccount(u.id, { canSharePublicly: u.canSharePublicly });
					return u.canSharePublicly;
				}
			);
			if (!latest) return;
			toast.success(
				next ? m.users_share_enabled({ username }) : m.users_share_disabled({ username })
			);
		} catch (error) {
			refused(error);
		}
	}

	async function rename(name: string) {
		if (!person) return;
		const updated = await updateUser(person.id, { displayName: name });
		profileSaved(updated);
		toast.success(m.person_renamed({ name: updated.displayName }));
	}

	async function saveEmail(email: string) {
		if (!person) return {};
		const saved = await updateUser(person.id, { email });
		profileSaved(saved);
		return saved;
	}

	const isBadAddress = (error: unknown) =>
		error instanceof ApiError &&
		error.status === 422 &&
		error.errors.some((e) => e.location === 'body.email');

	/**
	 * Issuing replaces any open link, the same act as a password reset, so it
	 * needs no confirmation - the dialog that follows is confirmation enough.
	 * With mail on, a step first asks where the link goes.
	 */
	function askSetupLink() {
		if (mailEnabled) sendOpen = true;
		else issueAndShow({ mail: false }).catch(() => {});
	}

	/**
	 * Issues a setup link and opens its result. Toasts a failure and rethrows
	 * it, so the send step knows to close. A refused address is the step's to
	 * show under its field instead.
	 */
	async function issueAndShow(opts: { mail: boolean; email?: string }): Promise<void> {
		if (!person) return;
		let link: SetupLinkInfo;
		try {
			link = await issueSetupLink(person.id, opts);
		} catch (error) {
			if (!isBadAddress(error)) refused(error);
			throw error;
		}
		patchAccount(person.id, {
			setupLinkExpiresAt: link.expiresAt,
			// A changed address is unconfirmed until the link is redeemed;
			// the service keeps a spelling that differs only in case.
			...(opts.email !== undefined &&
			opts.email.toLowerCase() !== (setup?.email ?? '').toLowerCase()
				? { email: opts.email, emailVerified: false }
				: {})
		});
		setupLinkInfo = link;
		// Which mail the address got changed with the link; the email row
		// reads it from the account.
		void load(true);
		// The step closes a tick before the result opens: two dialogs flipped
		// in one update leave both open, the step stuck behind the result.
		if (sendOpen) {
			sendOpen = false;
			await tick();
		}
		setupLinkOpen = true;
	}

	/**
	 * No confirmation: a revoked link is replaced in one click, and issuing a
	 * fresh one is right there if it was a mistake. "Manage" leaves with the
	 * link; "Create link" in its place takes focus.
	 */
	async function revokeLink() {
		if (!person) return;
		try {
			await revokeSetupLink(person.id);
			toast.success(m.users_setup_link_revoked());
			await load(true);
			await tick();
			focusSetupControl();
		} catch (error) {
			if (!isSignedOut(error)) {
				toast.error(m.users_update_error());
			}
		}
	}

	/**
	 * Confirmed first, like a delete: it signs the person out everywhere, and
	 * without a password they are locked out until they get a setup link.
	 */
	async function unlinkIdentity() {
		if (!person) return;
		try {
			await unlinkUserIdentity(person.id);
			patchAccount(person.id, { hasIdentity: false });
			toast.success(m.users_identity_unlinked({ name: provider ?? '', username: person.username }));
		} catch (error) {
			refused(error);
		}
	}

	// The page is the account's; with the account gone, the list is the
	// nearest place that still stands.
	async function remove() {
		if (!person) return;
		const { id, username } = person;
		try {
			await deleteUser(id);
			await goto(resolve('/settings/users'), { replaceState: true });
			toast.success(m.users_deleted({ username }));
		} catch (error) {
			if (error instanceof ApiError && error.status === 409) {
				toast.error(m.users_delete_conflict());
			} else if (error instanceof ApiError && error.status === 403) {
				toast.error(m.users_rank_required());
			} else if (!isSignedOut(error)) {
				toast.error(m.users_delete_error());
			}
		}
	}

	// The setup-link row's one control, whichever it is right now.
	function focusSetupControl() {
		document.getElementById(`${uid}-setup`)?.querySelector('button')?.focus();
	}

	// A dialog gives focus back to the control that opened it. Two of those
	// are gone by then: "Create link" turns into "Manage" once a link is
	// open, and the disconnect row leaves with the identity. Focus would
	// fall to the top of the page; the setup-link control takes it instead.
	let dialogWasOpen = false;
	$effect(() => {
		if (setupLinkOpen || sendOpen || unlinkOpen) {
			dialogWasOpen = true;
			return;
		}
		if (!dialogWasOpen) return;
		dialogWasOpen = false;
		// ponytail: waits out the dialog's 200ms close; a dialog close event would be exact.
		const timer = setTimeout(() => {
			if (!document.activeElement || document.activeElement === document.body) {
				focusSetupControl();
			}
		}, 300);
		return () => clearTimeout(timer);
	});

	const menuItemClass =
		'flex h-10 items-center rounded-sm px-3 text-body-sm text-text outline-none data-highlighted:bg-background';
</script>

<svelte:head>
	<title>{person?.displayName ?? data.username} · {m.settings_nav_users()} · {m.app_name()}</title>
</svelte:head>

<SettingsLayout active="users">
	<!-- Where this page sits, and the way back: the account is someone
	     else's, so the trail names them above the card. A long name wraps
	     under the link instead of pushing the card sideways. -->
	<nav
		aria-label={m.person_trail()}
		class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1 text-body-sm font-semibold"
	>
		<a
			href={resolve('/settings/users')}
			class="relative inline-flex shrink-0 items-center gap-1 self-start rounded-sm text-text-muted transition after:absolute after:inset-x-0 after:-inset-y-3 hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			<ChevronLeft class="size-4" aria-hidden="true" />
			{m.settings_nav_users()}
		</a>
		{#if person && permissions}
			<span aria-hidden="true" class="text-text-muted">/</span>
			<span aria-current="page" class="min-w-0 break-words hyphens-auto">{person.displayName}</span>
		{/if}
	</nav>

	{#if loading}
		<div class="overflow-hidden rounded-2xl bg-surface">
			<Skeleton class="h-28 rounded-none" />
			<div class="space-y-3 p-6">
				<Skeleton class="mx-auto h-8 w-48 rounded-md" />
				<Skeleton class="h-12 rounded-md" />
				<Skeleton class="h-12 rounded-md" />
			</div>
		</div>
	{:else if loadFailed}
		<section class="flex flex-col items-center gap-4 rounded-2xl bg-surface px-6 py-10 text-center">
			<p class="text-body text-text-muted">{m.users_load_error()}</p>
			<Button variant="secondary" onclick={() => load()}>{m.common_retry()}</Button>
		</section>
	{:else if verdict.kind === 'missing'}
		<section class="rounded-2xl bg-surface">
			<EmptyState
				title={m.person_missing_title()}
				text={m.person_missing_text({ username: data.username })}
			>
				<Button variant="secondary" href={resolve('/settings/users')}>
					{m.person_missing_back()}
				</Button>
			</EmptyState>
		</section>
	{:else if person && permissions}
		<section aria-label={person.displayName} class="overflow-hidden rounded-2xl bg-surface">
			<!-- The head reads the color the picker holds, so the band follows a
			     pick before its debounced save lands. -->
			<ProfileHead
				person={{ ...person, color }}
				roleLabel={roleLabel(person.role)}
				editable={permissions.editProfile}
				upload={(file, crop) => setUserAvatar(person.id, file, crop)}
				removeAvatar={() => removeUserAvatar(person.id)}
				onavatar={(avatarId) => replace({ ...person, avatarId })}
				onrename={rename}
				onerror={refused}
			>
				{#snippet colors()}
					<ColorPicker
						bind:value={color}
						{people}
						selfId={person.id}
						label={m.users_field_color()}
						variant="head"
						onchange={changeColor}
					/>
				{/snippet}
			</ProfileHead>

			<div class="grid grid-cols-1 gap-7 px-5 pt-7 pb-3 md:px-7 md:pb-4">
				<SettingsGroup label={m.person_group_overview()}>
					<SettingsRow icon={BookOpen} label={recipesText(person.recipeCount ?? 0)}>
						{#snippet value()}
							<!-- Newest created, as the people list's strip shows it. -->
							{#if person.latestRecipes?.[0]}
								<!-- A title hyphenates, so a long one keeps its closing quote. -->
								<span class="hyphens-auto"
									>{m.users_detail_last_recipe({ title: person.latestRecipes[0].title })}</span
								>
							{/if}
						{/snippet}
					</SettingsRow>
					<SettingsRow icon={ShoppingBasket} label={listsText(person.listCount ?? 0)} />
					{#if joined}
						<SettingsRow icon={Calendar} label={m.person_joined({ month: joined })} />
					{/if}
					{#if activity}
						<SettingsRow
							icon={Activity}
							label={activity.text}
							tone={activity.dormant ? 'attention' : 'default'}
						/>
					{/if}
				</SettingsGroup>

				{#if permissions.manageAccount}
					<SettingsGroup label={m.person_group_account()}>
						<EmailRow
							email={setup?.email ?? ''}
							emailVerified={setup?.emailVerified ?? false}
							emailConfirmation={setup?.emailConfirmation}
							emailSetupLink={setup?.emailSetupLink}
							mailOn={mailEnabled}
							resend={() => resendUserEmailConfirmation(person.id)}
							onchanged={() => void load(true)}
							none={m.users_detail_email_none()}
						>
							{#snippet control()}
								<Button
									variant="quiet"
									size="sm"
									label={m.person_email_change_aria({ name: person.displayName })}
									onclick={() => (emailOpen = true)}
								>
									{m.person_change()}
								</Button>
							{/snippet}
						</EmailRow>
						<SettingsRow icon={KeyRound} label={m.person_password()}>
							{#snippet value()}{m.person_reset_hint({ name: firstName })}{/snippet}
							{#snippet control()}
								<Button
									variant="quiet"
									size="sm"
									label={m.person_reset_aria({ name: person.displayName })}
									onclick={() => (resetOpen = true)}
								>
									{m.users_reset_submit()}
								</Button>
							{/snippet}
						</SettingsRow>
						<SettingsRow icon={Link} label={m.users_setup_link_action()}>
							{#snippet value()}{signIn}{/snippet}
							{#snippet control()}
								<span id="{uid}-setup" class="contents">
									{#if setup?.setupLinkExpiresAt}
										<DropdownMenu.Root>
											<DropdownMenu.Trigger>
												{#snippet child({ props })}
													<Button
														{...props}
														variant="quiet"
														size="sm"
														label={m.person_setup_link_manage_aria()}
													>
														{m.person_setup_link_manage()}
														<ChevronDown class="size-4" aria-hidden="true" />
													</Button>
												{/snippet}
											</DropdownMenu.Trigger>
											<DropdownMenu.Portal>
												<DropdownMenu.Content
													preventScroll={false}
													sideOffset={8}
													collisionPadding={16}
													align="end"
													class="w-56 max-w-[calc(100vw-2rem)] rounded-2xl border border-popover-border bg-popover p-2 shadow-dialog"
												>
													<DropdownMenu.Item onSelect={() => askSetupLink()} class={menuItemClass}>
														{m.person_setup_link_new()}
													</DropdownMenu.Item>
													<DropdownMenu.Item onSelect={() => revokeLink()} class={menuItemClass}>
														{m.person_setup_link_revoke()}
													</DropdownMenu.Item>
												</DropdownMenu.Content>
											</DropdownMenu.Portal>
										</DropdownMenu.Root>
									{:else}
										<Button
											variant="quiet"
											size="sm"
											label={m.person_setup_link_create_aria({ name: person.displayName })}
											onclick={() => askSetupLink()}
										>
											{m.person_setup_link_create()}
										</Button>
									{/if}
								</span>
							{/snippet}
						</SettingsRow>
						{#if setup?.hasIdentity && provider}
							{@const name = provider}
							<SettingsRow icon={Unlink} label={m.settings_signin_title()}>
								{#snippet value()}{m.oidc_linked({ name })}{/snippet}
								{#snippet control()}
									<Button
										variant="quiet"
										size="sm"
										label={m.users_identity_unlink({ name })}
										onclick={() => (unlinkOpen = true)}
									>
										{m.settings_signin_disconnect()}
									</Button>
								{/snippet}
							</SettingsRow>
						{/if}
					</SettingsGroup>
				{/if}

				<SettingsGroup label={m.person_group_rights()}>
					<SettingsRow icon={Shield} label={m.users_field_role()}>
						{#snippet control()}
							{#if permissions.changeRole}
								<Select
									bind:value={role}
									options={roleOptions}
									label={m.users_role_aria({ username: person.username })}
									onchange={changeRole}
									variant="pill"
									align="end"
									class="h-9 bg-background"
								/>
							{:else}
								<span class="text-body-sm text-text-muted">{roleLabel(person.role)}</span>
							{/if}
						{/snippet}
					</SettingsRow>
					{#if permissions.toggleSharing}
						<SettingsRow icon={Globe} label={m.users_share_toggle()}>
							{#snippet value()}{m.person_share_hint()}{/snippet}
							{#snippet control()}
								<Switch checked={sharing} label={m.users_share_toggle()} onchange={changeSharing} />
							{/snippet}
						</SettingsRow>
					{/if}
				</SettingsGroup>

				{#if permissions.manageAccount}
					<div class="border-t border-border">
						<SettingsRow icon={Trash2} label={m.users_delete_account()} tone="danger">
							{#snippet value()}{m.person_delete_hint()}{/snippet}
							{#snippet control()}
								<Button
									variant="destructive-quiet"
									size="sm"
									label={m.users_delete_account()}
									onclick={() => (deleteOpen = true)}
								>
									{m.common_delete()}
								</Button>
							{/snippet}
						</SettingsRow>
					</div>
				{/if}
			</div>
		</section>
	{/if}
</SettingsLayout>

{#if person && permissions?.manageAccount}
	<EmailDialog
		bind:open={emailOpen}
		email={setup?.email ?? ''}
		sentAt={setup?.emailConfirmation?.sentAt}
		name={firstName}
		mailOn={mailEnabled}
		save={saveEmail}
		onerror={refused}
	/>
{/if}
<!-- Mounted while the page is, so the close transition can play. -->
<ResetPasswordDialog
	bind:open={resetOpen}
	user={person ?? null}
	onreset={(user) => patchAccount(user.id, { hasPassword: true })}
/>
<SetupLinkDialog
	bind:open={setupLinkOpen}
	link={setupLinkInfo}
	displayName={person?.displayName ?? ''}
/>
<SendSetupLinkDialog
	bind:open={sendOpen}
	displayName={person?.displayName ?? ''}
	email={setup?.email ?? ''}
	emailVerified={setup?.emailVerified ?? false}
	onsend={(address) => issueAndShow({ mail: true, email: address })}
	onshowonly={() => issueAndShow({ mail: false })}
/>
<ConfirmDialog
	bind:open={deleteOpen}
	title={m.users_delete_confirm_title()}
	text={m.users_delete_confirm_text({ username: person?.username ?? '' })}
	confirmLabel={m.common_delete()}
	destructive
	onconfirm={remove}
/>
<ConfirmDialog
	bind:open={unlinkOpen}
	title={m.users_identity_unlink_confirm_title({ name: provider ?? '' })}
	text={m.users_identity_unlink_confirm_text({ username: person?.username ?? '' })}
	confirmLabel={m.users_identity_unlink({ name: provider ?? '' })}
	destructive
	onconfirm={unlinkIdentity}
/>
