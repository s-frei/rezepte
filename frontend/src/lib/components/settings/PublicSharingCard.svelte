<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { getSettings, updateSettings, type Settings } from '$lib/api/settings';
	import { session } from '$lib/auth.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import {
		lifetimeFromKey,
		lifetimeKey,
		lifetimeLabel,
		lifetimeOptions
	} from '$lib/recipe/lifetimes';
	import { m } from '$lib/paraglide/messages';

	let { ownerName }: { ownerName: string } = $props();

	const isOwner = $derived(session.user?.role === 'superadmin');
	let current = $state<Pick<
		Settings,
		'publicShares' | 'publicShareDefaultDays' | 'publicShareMaxDays' | 'publicShareAttribution'
	> | null>(null);

	// The maximum offers all five lifetimes; the default is capped at
	// whatever the maximum currently is, so it can never name a lifetime the
	// maximum itself forbids.
	const maxOptions = lifetimeOptions(null).map((days) => ({
		value: lifetimeKey(days),
		label: lifetimeLabel(days)
	}));
	const defaultOptions = $derived(
		(current ? lifetimeOptions(current.publicShareMaxDays) : []).map((days) => ({
			value: lifetimeKey(days),
			label: lifetimeLabel(days)
		}))
	);

	// The selects' own text values. Writable derived (Svelte >= 5.25): each
	// follows `current` and resyncs whenever it changes - including a
	// rejected write's rollback and a lowered maximum pulling the default
	// down with it - but a pick updates its own value first, through the
	// select's two-way binding, the same instant the write goes out.
	let defaultKey = $derived(current ? lifetimeKey(current.publicShareDefaultDays) : '');
	let maxKey = $derived(current ? lifetimeKey(current.publicShareMaxDays) : '');

	// Turning sharing on puts recipes in front of anyone holding a link, so it
	// asks first; turning it off takes effect at once. The switch flips under
	// the pointer before the question is answered, so it is re-created
	// whenever the dialog closes - it then shows `current` again, on or off.
	let confirmOpen = $state(false);
	let switchKey = $state(0);

	function flip(on: boolean) {
		if (on) {
			confirmOpen = true;
		} else {
			void save({ publicShares: false });
		}
	}

	function setConfirmOpen(open: boolean) {
		confirmOpen = open;
		if (!open) switchKey++;
	}

	onMount(async () => {
		try {
			const s = await getSettings();
			current = {
				publicShares: s.publicShares,
				publicShareDefaultDays: s.publicShareDefaultDays,
				publicShareMaxDays: s.publicShareMaxDays,
				publicShareAttribution: s.publicShareAttribution
			};
		} catch {
			current = null;
		}
	});

	// Applies on change, like the other switches here: the switch flips at
	// once, and a lifetime select already shows its own new pick through its
	// own two-way binding. Sends only the field that changed and shows the
	// server's full answer afterwards, since lowering the maximum below the
	// default lowers the default with it; a failed write snaps back.
	async function save(patch: Partial<Settings>) {
		const before = current;
		if (current) {
			current = { ...current, ...patch };
		}
		try {
			const s = await updateSettings(patch);
			current = {
				publicShares: s.publicShares,
				publicShareDefaultDays: s.publicShareDefaultDays,
				publicShareMaxDays: s.publicShareMaxDays,
				publicShareAttribution: s.publicShareAttribution
			};
			toast.success(m.settings_public_sharing_saved());
		} catch {
			current = before;
			toast.error(m.settings_public_sharing_error());
		}
	}
</script>

<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-public-sharing">
	<h2 id="settings-public-sharing" class="mb-4 font-display text-heading font-medium">
		{m.settings_public_sharing_title()}
	</h2>
	{#if current !== null}
		{#if isOwner}
			<div class="flex items-start justify-between gap-4">
				<div class="min-w-0">
					<p class="text-body font-semibold">{m.settings_public_sharing_switch()}</p>
					<p class="mt-1 max-w-[60ch] text-caption text-text-muted">
						{m.settings_public_sharing_hint()}
					</p>
				</div>
				{#key switchKey}
					<Switch
						checked={current.publicShares}
						label={m.settings_public_sharing_switch()}
						onchange={flip}
					/>
				{/key}
			</div>
			{#if current.publicShares}
				<div class="mt-5 flex flex-col gap-4 sm:flex-row sm:gap-6">
					<div class="min-w-0 flex-1">
						<p class="mb-1.5 text-body-sm font-semibold">
							{m.settings_public_sharing_default_label()}
						</p>
						<Select
							bind:value={defaultKey}
							options={defaultOptions}
							label={m.settings_public_sharing_default_label()}
							onchange={(next) => save({ publicShareDefaultDays: lifetimeFromKey(next) })}
							class="w-full bg-surface-elevated"
						/>
					</div>
					<div class="min-w-0 flex-1">
						<p class="mb-1.5 text-body-sm font-semibold">{m.settings_public_sharing_max_label()}</p>
						<Select
							bind:value={maxKey}
							options={maxOptions}
							label={m.settings_public_sharing_max_label()}
							onchange={(next) => save({ publicShareMaxDays: lifetimeFromKey(next) })}
							class="w-full bg-surface-elevated"
						/>
					</div>
				</div>
			{/if}
			<div class="mt-5 flex items-start justify-between gap-4 border-t border-border pt-5">
				<div class="min-w-0">
					<p class="text-body font-semibold">{m.settings_public_sharing_attribution()}</p>
					<p class="mt-1 max-w-[60ch] text-caption text-text-muted">
						{m.settings_public_sharing_attribution_hint()}
					</p>
				</div>
				<Switch
					checked={current.publicShareAttribution}
					label={m.settings_public_sharing_attribution()}
					onchange={(on) => save({ publicShareAttribution: on })}
				/>
			</div>
		{:else if ownerName}
			<!-- Waits for the members list: without it the sentence has no one to name. -->
			<p class="max-w-[60ch] text-body-sm text-text-muted">
				{current.publicShares
					? m.settings_public_sharing_state_on({
							max: lifetimeLabel(current.publicShareMaxDays),
							owner: ownerName
						})
					: m.settings_public_sharing_state_off({ owner: ownerName })}
			</p>
		{/if}
	{/if}
</section>

<ConfirmDialog
	bind:open={() => confirmOpen, setConfirmOpen}
	title={m.settings_public_sharing_confirm_title()}
	text={m.settings_public_sharing_confirm_text()}
	confirmLabel={m.settings_public_sharing_confirm()}
	onconfirm={() => save({ publicShares: true })}
/>
