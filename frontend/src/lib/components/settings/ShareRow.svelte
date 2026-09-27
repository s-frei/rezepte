<script lang="ts">
	import { resolve } from '$app/paths';
	import Copy from '@lucide/svelte/icons/copy';
	import type { PublicShare } from '$lib/api/shares';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatDate } from '$lib/recipe/format';
	import { copyLink } from '$lib/recipe/share.svelte';
	import { shareStatusLabel } from '$lib/recipe/share-status';
	import { userColorClasses } from '$lib/user/color';

	let {
		share,
		/** The admin "everyone" view: names who created each link. */
		showCreator = false,
		/** Rejects (and leaves the row in place) when the API refused. */
		onrevoke
	}: {
		share: PublicShare;
		showCreator?: boolean;
		onrevoke: (share: PublicShare) => Promise<void>;
	} = $props();

	let revokeOpen = $state(false);

	const url = $derived(location.origin + share.path);
	const initial = $derived(share.createdBy?.displayName.charAt(0).toUpperCase() ?? '');

	async function revoke() {
		await onrevoke(share);
	}
</script>

<li
	class="flex flex-col gap-2 border-b border-dashed border-border py-4 last:border-b-0 md:flex-row md:items-center md:justify-between md:gap-4"
>
	<div class="min-w-0">
		<a
			href={resolve('/recipes/[slug]', { slug: share.recipe.slug })}
			class="truncate font-display text-body font-medium text-text hover:text-primary"
		>
			{share.recipe.title}
		</a>
		<p class="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-text-muted">
			<span>{m.settings_shares_created({ date: formatDate(share.createdAt) })}</span>
			<span aria-hidden="true">·</span>
			<span>
				{share.expiresAt
					? m.public_share_valid_until({ date: formatDate(share.expiresAt) })
					: m.public_share_lifetime_permanent()}
			</span>
			<span aria-hidden="true">·</span>
			<span>{shareStatusLabel(share.status)}</span>
			{#if showCreator && share.createdBy}
				{@const creator = share.createdBy}
				<span aria-hidden="true">·</span>
				<span class="inline-flex items-center gap-1.5">
					<span
						aria-hidden="true"
						class="flex size-4 items-center justify-center rounded-full initial-centered font-display text-label font-semibold tracking-normal {userColorClasses(
							creator.color
						)}"
					>
						{initial}
					</span>
					{m.settings_shares_by({ name: creator.displayName })}
				</span>
			{/if}
		</p>
	</div>
	<div class="flex shrink-0 items-center gap-2">
		<Button
			variant="secondary"
			class="h-11 px-3 text-caption whitespace-nowrap md:h-8"
			onclick={() => copyLink(url)}
		>
			<Copy class="size-4" aria-hidden="true" />
			{m.public_share_copy()}
		</Button>
		<button
			type="button"
			onclick={() => (revokeOpen = true)}
			class="inline-flex h-11 items-center rounded-pill px-3 text-caption font-semibold whitespace-nowrap text-destructive transition hover:bg-destructive-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:h-8"
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
