<script lang="ts">
	import { Dialog } from 'bits-ui';
	import CopyIcon from '$lib/components/icons/CopyIcon.svelte';
	import Share2 from '@lucide/svelte/icons/share-2';
	import { toast } from 'svelte-sonner';
	import { ApiError } from '$lib/api/client';
	import {
		createPublicShare,
		revokeShare,
		type PublicShare,
		type ShareLifetime,
		type ShareRefusal
	} from '$lib/api/shares';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { formatDate } from '$lib/recipe/format';
	import {
		lifetimeFromKey,
		lifetimeKey,
		lifetimeLabel,
		lifetimeOptions
	} from '$lib/recipe/lifetimes';
	import { copyLink, shareLink } from '$lib/recipe/share.svelte';
	import { shareStatusLabel } from '$lib/recipe/share-status';
	import { m } from '$lib/paraglide/messages';

	let {
		open = $bindable(false),
		recipeId,
		recipeTitle,
		defaultDays,
		maxDays,
		share = $bindable(null)
	}: {
		open?: boolean;
		recipeId: string;
		recipeTitle: string;
		/** `settings.publicShareDefaultDays` - preselected when there is no link yet. */
		defaultDays: ShareLifetime;
		/** `settings.publicShareMaxDays` - caps the lifetime choice. */
		maxDays: ShareLifetime;
		/** The caller's current public link for this recipe, if any. Two-way so
		 * creating or revoking one here is reflected in the recipe page's
		 * marker and menu item at once, without a reload. */
		share?: PublicShare | null;
	} = $props();

	const options = $derived(lifetimeOptions(maxDays));
	const canShareSheet = typeof navigator !== 'undefined' && Boolean(navigator.share);
	const linkUrl = $derived(share ? location.origin + share.path : '');

	let selectedKey = $state('');
	let creating = $state(false);
	let revokeOpen = $state(false);

	// A fresh choice every time the create form opens - the default only
	// matters until the first pick, but a dialog reopened later (after a
	// revoke) should propose it again rather than keep whatever was last
	// selected.
	$effect(() => {
		if (open && !share) {
			const initial = options.includes(defaultDays) ? defaultDays : (options[0] ?? null);
			selectedKey = lifetimeKey(initial);
		}
	});

	async function create() {
		creating = true;
		try {
			share = await createPublicShare(recipeId, lifetimeFromKey(selectedKey));
		} catch (error) {
			if (error instanceof ApiError && error.status === 409) {
				// A second tab (or a double click) got there first: the problem
				// document carries that existing link, so this just shows it
				// instead of failing.
				const existing = error.body.share as PublicShare | undefined;
				if (existing) {
					share = existing;
					return;
				}
			}
			if (error instanceof ApiError && error.status === 403) {
				const reason = error.body.reason as ShareRefusal | undefined;
				toast.error(
					reason === 'sharing-off' ? m.public_share_error_off() : m.public_share_error_not_allowed()
				);
				return;
			}
			if (error instanceof ApiError && error.status === 422) {
				toast.error(m.public_share_error_lifetime());
				return;
			}
			toast.error(m.public_share_create_error());
		} finally {
			creating = false;
		}
	}

	async function revoke() {
		if (!share) {
			return;
		}
		try {
			await revokeShare(share.id);
			share = null;
			toast.success(m.public_share_revoked());
			open = false;
		} catch {
			toast.error(m.public_share_revoke_error());
		}
	}

	let copyIcon = $state<CopyIcon>();
</script>

<BaseDialog bind:open>
	<Dialog.Title class="font-display text-heading font-medium">
		{m.public_share_dialog_title()}
	</Dialog.Title>
	{#if share}
		{@const link = share}
		<Dialog.Description class="sr-only">{m.public_share_dialog_description()}</Dialog.Description>
		<div class="mt-5 flex items-center gap-2 rounded-md bg-surface-elevated p-3">
			<span class="min-w-0 flex-1 truncate text-body-sm">{linkUrl}</span>
		</div>
		<div class="mt-2 flex flex-wrap items-center gap-2 text-caption text-text-muted">
			<span>
				{link.expiresAt
					? m.public_share_valid_until({ date: formatDate(link.expiresAt) })
					: m.public_share_lifetime_permanent()}
			</span>
			{#if link.status !== 'active'}
				<span class="rounded-pill bg-surface-elevated px-2 py-0.5 text-micro font-semibold">
					{shareStatusLabel(link.status)}
				</span>
			{/if}
		</div>
		<div class="mt-3 flex flex-wrap gap-2">
			<Button
				variant="secondary"
				onclick={async () => {
					if (await copyLink(linkUrl)) copyIcon?.play();
				}}
			>
				<CopyIcon bind:this={copyIcon} class="size-4" />
				{m.public_share_copy()}
			</Button>
			{#if canShareSheet}
				<Button variant="secondary" onclick={() => shareLink(recipeTitle, linkUrl)}>
					<Share2 class="size-4" aria-hidden="true" />
					{m.public_share_share()}
				</Button>
			{/if}
		</div>
		<div class="mt-6 flex justify-end">
			<Button variant="destructive" onclick={() => (revokeOpen = true)}>
				{m.public_share_revoke()}
			</Button>
		</div>
	{:else}
		<Dialog.Description class="mt-2 text-body text-text-muted">
			{m.public_share_dialog_description()}
		</Dialog.Description>
		<div class="mt-5 flex flex-col gap-1.5">
			<span class="text-body-sm font-semibold">{m.public_share_lifetime_label()}</span>
			<Select
				bind:value={selectedKey}
				options={options.map((days) => ({ value: lifetimeKey(days), label: lifetimeLabel(days) }))}
				label={m.public_share_lifetime_label()}
				class="w-full bg-surface-elevated"
			/>
		</div>
		<div class="mt-6 flex justify-end">
			<Button onclick={create} disabled={creating}>{m.public_share_create()}</Button>
		</div>
	{/if}
</BaseDialog>

<ConfirmDialog
	bind:open={revokeOpen}
	title={m.public_share_revoke_title()}
	text={m.public_share_revoke_text()}
	confirmLabel={m.public_share_revoke()}
	destructive
	onconfirm={revoke}
/>
