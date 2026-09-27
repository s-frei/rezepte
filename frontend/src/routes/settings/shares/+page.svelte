<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import CircleAlert from '@lucide/svelte/icons/circle-alert';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import {
		listShares,
		revokeAllShares,
		revokeShare,
		type PublicShare,
		type ShareCreator,
		type ShareStatus
	} from '$lib/api/shares';
	import { getSettings } from '$lib/api/settings';
	import { isSignedOut } from '$lib/api/client';
	import { session } from '$lib/auth.svelte';
	import SettingsLayout from '$lib/components/settings/SettingsLayout.svelte';
	import ShareRow from '$lib/components/settings/ShareRow.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import TagChip from '$lib/components/ui/TagChip.svelte';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import { shareStatusLabel } from '$lib/recipe/share-status';
	import { isAdminRole } from '$lib/roles';
	import { userColorClasses } from '$lib/user/color';

	const isAdmin = $derived(isAdminRole(session.user?.role));
	const isOwner = $derived(session.user?.role === 'superadmin');

	// Admins can list everyone's links with their creators instead of their
	// own alone, and anyone can narrow the list by status; admins in the
	// everyone view also by person. All three are view filters, not settings:
	// they live in the URL (?all=1&user=<id>&status=paused), so a reload or a
	// shared address keeps them, and opening the page from the navigation
	// starts with the viewer's own, unfiltered list.
	let everyone = $state(page.url.searchParams.get('all') === '1');
	let person = $state(page.url.searchParams.get('user') ?? '');
	let status = $state(page.url.searchParams.get('status') ?? '');
	let shares = $state<PublicShare[]>([]);
	let loading = $state(true);
	let loadFailed = $state(false);
	let revokeAllOpen = $state(false);
	// Whether sharing is on for the household, for the notice above the list;
	// null until known, and when it cannot be read - then there is no notice.
	let sharingOn = $state<boolean | null>(null);

	async function load() {
		loading = true;
		loadFailed = false;
		try {
			shares = await listShares(isAdmin && everyone);
		} catch (error) {
			// On a 401 the client is already navigating to the login page.
			loadFailed = !isSignedOut(error);
		} finally {
			loading = false;
		}
	}

	const statusOrder: ShareStatus[] = ['active', 'paused', 'limited'];

	// A filter value from the URL that matches no link (any more) filters
	// nothing, so revoking someone's last link never leaves an empty list.
	const selectedPerson = $derived(
		shares.some((share) => share.createdBy?.id === person) ? person : ''
	);
	const selectedStatus = $derived(
		shares.some((share) => share.status === status) ? (status as ShareStatus) : ''
	);
	const byPerson = $derived(
		selectedPerson ? shares.filter((share) => share.createdBy?.id === selectedPerson) : shares
	);
	const byStatus = $derived(
		selectedStatus ? shares.filter((share) => share.status === selectedStatus) : shares
	);
	const visible = $derived(
		selectedStatus ? byPerson.filter((share) => share.status === selectedStatus) : byPerson
	);

	// Everyone who has a link, by the ID the API gives - display names need
	// not be unique. Each count is within the chosen status, so the numbers
	// always say what a click would show.
	const creators = $derived.by(() => {
		const list: (ShareCreator & { count: number })[] = [];
		for (const share of shares) {
			const creator = share.createdBy;
			if (creator && !list.some((c) => c.id === creator.id)) {
				list.push({ ...creator, count: 0 });
			}
		}
		for (const share of byStatus) {
			const entry = list.find((c) => c.id === share.createdBy?.id);
			if (entry) entry.count++;
		}
		return list.sort((a, b) => a.displayName.localeCompare(b.displayName, getLocale()));
	});
	// The statuses the list holds, counted within the chosen person.
	const statuses = $derived(
		statusOrder
			.filter((s) => shares.some((share) => share.status === s))
			.map((s) => ({
				status: s,
				count: byPerson.filter((share) => share.status === s).length
			}))
	);

	onMount(() => {
		void load();
		getSettings()
			.then((s) => (sharingOn = s.publicShares))
			.catch(() => (sharingOn = null));
	});

	function syncUrl() {
		const params: string[] = [];
		if (everyone) {
			params.push('all=1');
			if (person) params.push(`user=${encodeURIComponent(person)}`);
		}
		if (status) params.push(`status=${encodeURIComponent(status)}`);
		const qs = params.join('&');
		void goto(resolve(qs ? `/settings/shares?${qs}` : '/settings/shares'), {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}

	function toggleEveryone(on: boolean) {
		everyone = on;
		if (!on) person = '';
		syncUrl();
		void load();
	}

	function pickPerson(id: string) {
		person = selectedPerson === id ? '' : id;
		syncUrl();
	}

	function pickStatus(next: ShareStatus) {
		status = selectedStatus === next ? '' : next;
		syncUrl();
	}

	async function revoke(share: PublicShare) {
		try {
			await revokeShare(share.id);
			shares = shares.filter((s) => s.id !== share.id);
			toast.success(m.public_share_revoked());
		} catch {
			toast.error(m.public_share_revoke_error());
		}
	}

	async function revokeAll() {
		try {
			await revokeAllShares();
			shares = [];
			toast.success(m.settings_shares_revoked_all());
		} catch {
			toast.error(m.public_share_revoke_error());
		}
	}
</script>

<svelte:head><title>{m.settings_nav_shares()} · {m.app_name()}</title></svelte:head>

<SettingsLayout active="shares">
	<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-shares">
		<div class="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between md:gap-4">
			<h2 id="settings-shares" class="font-display text-heading font-medium">
				{m.settings_nav_shares()}
			</h2>
			{#if isAdmin}
				<Button
					variant="secondary"
					onclick={() => (revokeAllOpen = true)}
					class="w-full whitespace-nowrap md:w-auto"
				>
					{m.settings_shares_revoke_all()}
				</Button>
			{/if}
		</div>
		<!-- Why the links below do not open, said where they are listed: the
		     household switch first, since it pauses everyone's; then the
		     viewer's own right. -->
		{#if sharingOn === false || session.user?.canSharePublicly === false}
			<div
				role="status"
				class="mb-4 flex items-start gap-2.5 rounded-md bg-accent px-4 py-3 text-body-sm text-accent-foreground"
			>
				<CircleAlert class="mt-0.5 size-4 shrink-0" aria-hidden="true" />
				<p>
					{#if sharingOn === false}
						{m.settings_shares_off()}
						{#if isOwner}
							<a
								href={resolve('/settings/users')}
								class="font-semibold underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
							>
								{m.settings_shares_off_owner()}
							</a>
						{:else}
							{m.settings_shares_off_other()}
						{/if}
					{:else}
						{m.settings_shares_withdrawn()}
					{/if}
				</p>
			</div>
		{/if}
		{#if isAdmin}
			<div class="mb-4 flex items-center justify-between gap-3 rounded-md bg-background px-4 py-3">
				<span class="text-body-sm font-medium">{m.settings_shares_everyone()}</span>
				<Switch checked={everyone} label={m.settings_shares_everyone()} onchange={toggleEveryone} />
			</div>
		{/if}
		<!-- Hidden below two people, like the overview's author filter: with one
		     creator there is nothing to pick between. -->
		{#if isAdmin && everyone && !loading && creators.length > 1}
			<section aria-labelledby="settings-shares-person" class="mb-4 space-y-2">
				<h3
					id="settings-shares-person"
					class="text-caption font-semibold text-text-muted uppercase"
				>
					{m.settings_shares_person_heading()}
				</h3>
				<div class="flex flex-wrap gap-2">
					{#each creators as creator (creator.id)}
						<TagChip
							label={creator.displayName}
							count={creator.count}
							active={selectedPerson === creator.id}
							onclick={() => pickPerson(creator.id)}
						>
							{#snippet leading()}
								<span
									aria-hidden="true"
									class="size-3 rounded-full {userColorClasses(creator.color)}"
								></span>
							{/snippet}
						</TagChip>
					{/each}
				</div>
			</section>
		{/if}
		<!-- Hidden below two statuses: a list that is all active has nothing to
		     narrow. -->
		{#if !loading && statuses.length > 1}
			<section aria-labelledby="settings-shares-status" class="mb-4 space-y-2">
				<h3
					id="settings-shares-status"
					class="text-caption font-semibold text-text-muted uppercase"
				>
					{m.settings_shares_status_heading()}
				</h3>
				<div class="flex flex-wrap gap-2">
					{#each statuses as entry (entry.status)}
						<TagChip
							label={shareStatusLabel(entry.status)}
							count={entry.count}
							active={selectedStatus === entry.status}
							onclick={() => pickStatus(entry.status)}
						/>
					{/each}
				</div>
			</section>
		{/if}
		{#if loading}
			<div class="space-y-3">
				<Skeleton class="h-14 rounded-md" />
				<Skeleton class="h-14 rounded-md" />
			</div>
		{:else if loadFailed}
			<div class="flex flex-col items-center gap-4 py-10 text-center">
				<p class="text-body text-text-muted">{m.settings_shares_load_error()}</p>
				<Button variant="secondary" onclick={load}>{m.common_retry()}</Button>
			</div>
		{:else if shares.length === 0}
			<p class="py-6 text-body-sm text-text-muted">{m.settings_shares_empty()}</p>
		{:else if visible.length === 0}
			<p class="py-6 text-body-sm text-text-muted">{m.settings_shares_no_match()}</p>
		{:else}
			<ul aria-label={m.settings_nav_shares()}>
				{#each visible as share (share.id)}
					<ShareRow {share} showCreator={isAdmin && everyone} onrevoke={revoke} />
				{/each}
			</ul>
		{/if}
	</section>
</SettingsLayout>

<ConfirmDialog
	bind:open={revokeAllOpen}
	title={m.settings_shares_revoke_all_title()}
	text={m.settings_shares_revoke_all_text()}
	confirmLabel={m.settings_shares_revoke_all()}
	destructive
	onconfirm={revokeAll}
/>
