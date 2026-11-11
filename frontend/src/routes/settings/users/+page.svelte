<script lang="ts">
	import { onMount } from 'svelte';
	import Lock from '@lucide/svelte/icons/lock';
	import Plus from '@lucide/svelte/icons/plus';
	import { isSignedOut } from '#lib/api/client.js';
	import { getOidc } from '#lib/api/oidc.js';
	import { getSettings, type Settings } from '#lib/api/settings.js';
	import {
		listPeople,
		listUsers,
		type PersonEntry,
		type SetupLinkInfo,
		type UserAccount
	} from '#lib/api/users.js';
	import { session } from '#lib/auth.svelte.js';
	import CreateUserDialog from '#lib/components/settings/CreateUserDialog.svelte';
	import LinkPreviewsCard from '#lib/components/settings/LinkPreviewsCard.svelte';
	import PublicSharingCard from '#lib/components/settings/PublicSharingCard.svelte';
	import RecipeEditingCard from '#lib/components/settings/RecipeEditingCard.svelte';
	import SettingsLayout from '#lib/components/settings/SettingsLayout.svelte';
	import SetupLinkDialog from '#lib/components/settings/SetupLinkDialog.svelte';
	import PeopleList from '#lib/components/settings/PeopleList.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Skeleton from '#lib/components/ui/Skeleton.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { isAdminRole } from '#lib/roles.js';
	import { rowMotion } from '#lib/settings/row-motion.svelte.js';

	let users = $state<PersonEntry[]>([]);
	const rows = rowMotion();
	let loading = $state(true);
	let loadFailed = $state(false);
	let createOpen = $state(false);

	// Every account reads this page; only an admin gets the controls, the
	// pickers that need the color counts, and the household's editing card.
	const isAdmin = $derived(isAdminRole(session.user?.role));

	// The list keeps only what it shows about each account and the link a
	// new account starts with; everything else happens on the person's page.
	let setup = $state<Record<string, UserAccount>>({});
	let provider = $state<string | undefined>(undefined);
	// Fetched once for the household cards below and the mail-aware dialogs.
	let settings = $state<Settings | null>(null);
	// Advisory like the provider: unknown means the dialogs work as without mail.
	const mailEnabled = $derived(settings?.mailEnabled ?? false);
	const ownerName = $derived(users.find((u) => u.role === 'superadmin')?.displayName ?? '');
	let setupLinkOpen = $state(false);
	let setupLinkInfo = $state<SetupLinkInfo | null>(null);
	let setupLinkName = $state('');

	// An inline error with a retry button instead of a toast over an empty
	// list: nobody can tell a failed load from "no one here", and a toast
	// leaves no way back onto the list short of reloading the page.
	async function load() {
		loading = true;
		loadFailed = false;
		try {
			const [list, accounts] = await Promise.all([
				listPeople(),
				isAdmin ? listUsers() : Promise.resolve([])
			]);
			users = list;
			setup = Object.fromEntries(accounts.map((a) => [a.id, a]));
		} catch (error) {
			// On a 401 the client is already navigating to the login page.
			loadFailed = !isSignedOut(error);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void load();
		// An extra, like the provider button on the login page: a failed
		// lookup leaves the action out.
		if (isAdmin) {
			getOidc()
				.then((info) => (provider = info.enabled ? info.name : undefined))
				.catch(() => {});
			getSettings()
				.then((s) => (settings = s))
				.catch(() => {});
		}
	});

	function userCreated(user: UserAccount, setupLink: SetupLinkInfo | null) {
		rows.play(() => (users = [...users, user]));
		// Recorded either way: a password-mode create has one already
		// (hasPassword true, no open link), and without this the new row
		// reads "Not set up yet" until the next reload.
		setup = {
			...setup,
			[user.id]: { ...user, hasPassword: !setupLink, setupLinkExpiresAt: setupLink?.expiresAt }
		};
		if (setupLink) {
			setupLinkInfo = setupLink;
			setupLinkName = user.displayName;
			setupLinkOpen = true;
		}
	}
</script>

<svelte:head><title>{m.settings_nav_users()} · {m.app_name()}</title></svelte:head>

<SettingsLayout active="users">
	<section class="rounded-2xl bg-surface p-6 max-sm:px-4 md:p-7" aria-labelledby="settings-users">
		<!-- Side by side the heading and the button need 306px, 4px more than a
		     360px phone leaves inside this card, and the label wrapped into two
		     lines. Below `md` the button gets its own full-width row instead. -->
		<div class="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between md:gap-4">
			<h2 id="settings-users" class="font-display text-heading font-medium">
				{m.settings_nav_users()}
			</h2>
			{#if isAdmin}
				<!-- Waits for the people: the form preselects the least worn color. -->
				<Button
					variant="accent"
					disabled={loading}
					onclick={() => (createOpen = true)}
					class="w-full text-center md:w-auto md:whitespace-nowrap"
				>
					<Plus class="size-4" aria-hidden="true" />
					{m.users_create()}
				</Button>
			{/if}
		</div>
		{#if !isAdmin}
			<p class="mb-4 text-body text-text-muted">{m.users_list_hint()}</p>
		{/if}
		{#if loading}
			<div class="space-y-3">
				<Skeleton class="h-12 rounded-md" />
				<Skeleton class="h-12 rounded-md" />
			</div>
		{:else if loadFailed}
			<div class="flex flex-col items-center gap-4 py-10 text-center">
				<p class="text-body text-text-muted">{m.users_load_error()}</p>
				<Button variant="secondary" onclick={load}>{m.common_retry()}</Button>
			</div>
		{:else}
			<PeopleList
				people={users}
				meId={session.user?.id ?? ''}
				actorRole={session.user?.role ?? 'user'}
				{setup}
				{provider}
				motion={rows.duration}
			/>
			<p class="mt-5 flex items-start gap-1.5 text-micro text-text-muted">
				<Lock class="mt-0.5 size-3 shrink-0" aria-hidden="true" />
				<span>{m.users_lists_privacy()}</span>
			</p>
		{/if}
	</section>
	{#if isAdmin}
		<RecipeEditingCard {ownerName} {settings} />
		<LinkPreviewsCard {ownerName} {settings} />
		<PublicSharingCard {ownerName} {settings} />
	{/if}
</SettingsLayout>

{#if isAdmin}
	<!-- The new account is appended, not sorted in: PeopleList owns the display
	     order and drops the row into its group. -->
	<CreateUserDialog bind:open={createOpen} people={users} {mailEnabled} oncreated={userCreated} />
	<!-- Opens right after Add account closes, for a fresh account with no
	     password. Stays mounted so its close transition can play. -->
	<SetupLinkDialog bind:open={setupLinkOpen} link={setupLinkInfo} displayName={setupLinkName} />
{/if}
