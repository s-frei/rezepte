<script lang="ts">
	import { resolve } from '$app/paths';
	import type { PublicShare } from '$lib/api/shares';
	import CopyIcon from '$lib/components/icons/CopyIcon.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import LifetimeBar from '$lib/components/ui/LifetimeBar.svelte';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';
	import Stamp from '$lib/components/ui/Stamp.svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/recipe/format';
	import { copyLink } from '$lib/recipe/share.svelte';
	import { shareStatusLabel } from '$lib/recipe/share-status';
	import { userColorEdge } from '$lib/user/color';

	let {
		share,
		/** The admin "everyone" view: names who created each link. */
		showCreator = false,
		/** The edge color when the API sends no creator - the viewer's own list. */
		ownColor,
		/** Whether the household switch is off, which is then why a link is paused. */
		sharingOff = false,
		/** Rejects (and leaves the row in place) when the API refused. */
		onrevoke
	}: {
		share: PublicShare;
		showCreator?: boolean;
		ownColor?: string;
		sharingOff?: boolean;
		onrevoke: (share: PublicShare) => Promise<void>;
	} = $props();

	let revokeOpen = $state(false);

	const url = $derived(location.origin + share.path);
	const dormant = $derived(share.status !== 'active');

	// A paused link is paused for one of two reasons, and the household switch
	// wins: the person's own right only matters while sharing is on.
	const reason = $derived(
		share.status === 'paused'
			? sharingOff
				? m.settings_shares_paused_household()
				: m.settings_shares_paused_person()
			: share.status === 'limited'
				? m.settings_shares_limited_reason()
				: ''
	);

	async function revoke() {
		await onrevoke(share);
	}

	let copyIcon = $state<CopyIcon>();
</script>

<!--
	The same key tag as an API token: the edge is the creator's color, the bar
	is how much of the link's life has passed. A dormant link - paused or
	limited, both reversible - keeps its dates readable and carries a stamp
	beside the sentence that says why; an expired link is gone from the list
	altogether, so there is no stamp for that.
-->
<li
	class="relative grid min-w-0 grid-cols-[minmax(0,1fr)] rounded-l-md rounded-r-2xl border border-l-[6px] border-border bg-surface-elevated py-4 pr-4 pl-8 md:pr-5 md:pl-10 {userColorEdge(
		share.createdBy?.color ?? ownColor
	)}"
>
	<span
		aria-hidden="true"
		class="absolute top-6 left-2.5 size-3 rounded-full bg-background shadow-inner md:left-3.5"
	></span>

	<div class="min-w-0 {dormant ? 'opacity-60' : ''}">
		<h3 class="font-display text-card leading-snug font-medium hyphens-auto">
			<a
				href={resolve('/recipes/[slug]', { slug: share.recipe.slug })}
				class="hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				{share.recipe.title}
			</a>
		</h3>
		{#if showCreator && share.createdBy}
			<p class="mt-1 flex items-center gap-1.5 text-caption font-semibold">
				<PersonMark size="xs" person={share.createdBy} />
				{share.createdBy.displayName}
			</p>
		{/if}
	</div>

	<div class="mt-3">
		<LifetimeBar item={share} faded={dormant} />
		<p class="sr-only">
			{m.settings_shares_created({ date: formatDate(share.createdAt) })}.
			{share.expiresAt
				? m.public_share_valid_until({ date: formatDate(share.expiresAt) })
				: m.settings_shares_permanent()}.
			{shareStatusLabel(share.status)}.
		</p>
	</div>

	{#if dormant}
		<div class="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2">
			<Stamp tone={share.status === 'limited' ? 'caution' : 'destructive'}>
				{shareStatusLabel(share.status)}
				<!-- A limited link's bar is spent, so its end date travels on the
					 stamp, as an expired token's does. -->
				{#if share.status === 'limited' && share.expiresAt}
					<span class="block font-sans text-micro font-semibold">
						{formatDate(share.expiresAt)}
					</span>
				{/if}
			</Stamp>
			<p class="text-caption text-text-muted">{reason}</p>
		</div>
	{/if}

	<div class="mt-4 flex flex-wrap items-center justify-between gap-2">
		<Button
			variant="secondary"
			class="h-11 px-4 text-caption whitespace-nowrap md:h-9"
			onclick={async () => {
				if (await copyLink(url)) copyIcon?.play();
			}}
		>
			<CopyIcon bind:this={copyIcon} class="size-4" />
			{m.settings_shares_copy()}
		</Button>
		<button
			type="button"
			onclick={() => (revokeOpen = true)}
			class="-mr-2 ml-auto inline-flex h-11 items-center rounded-pill px-3 text-caption font-semibold whitespace-nowrap text-text-muted transition hover:bg-destructive-soft hover:text-destructive focus-visible:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:h-8"
		>
			{m.public_share_revoke()}
		</button>
	</div>
</li>

<ConfirmDialog
	bind:open={revokeOpen}
	title={m.public_share_revoke_title()}
	text={m.public_share_revoke_text()}
	confirmLabel={m.public_share_revoke()}
	destructive
	onconfirm={revoke}
/>
