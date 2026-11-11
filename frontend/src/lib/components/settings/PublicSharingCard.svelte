<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { updateSettings, type Settings } from '#lib/api/settings.js';
	import { session } from '#lib/auth.svelte.js';
	import ConfirmDialog from '#lib/components/ui/ConfirmDialog.svelte';
	import Select from '#lib/components/ui/Select.svelte';
	import {
		lifetimeFromKey,
		lifetimeKey,
		lifetimeLabel,
		lifetimeOptions
	} from '#lib/recipe/lifetimes.js';
	import { optimistic } from '#lib/optimistic.js';
	import { m } from '#lib/paraglide/messages.js';
	import SettingsCard from './SettingsCard.svelte';
	import SwitchRow from './SwitchRow.svelte';

	let { ownerName, settings }: { ownerName: string; settings: Settings | null } = $props();

	const isOwner = $derived(session.user?.role === 'superadmin');
	const pick = (s: Settings) => ({
		publicShares: s.publicShares,
		publicShareDefaultDays: s.publicShareDefaultDays,
		publicShareMaxDays: s.publicShareMaxDays,
		publicShareAttribution: s.publicShareAttribution,
		publicShareSharer: s.publicShareSharer
	});
	let current = $derived(settings && pick(settings));

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

	// Applies on change, like the other switches here: the switch flips at
	// once, and a lifetime select already shows its own new pick through its
	// own two-way binding. Sends only the field that changed and shows the
	// server's full answer afterwards, since lowering the maximum below the
	// default lowers the default with it; a failed write snaps back to the
	// value the server holds (see `optimistic`).
	const send = optimistic(
		() => current,
		(value) => (current = value)
	);
	async function save(patch: Partial<Settings>) {
		try {
			const { latest } = await send(
				current && { ...current, ...patch },
				() => updateSettings(patch),
				pick
			);
			if (latest) toast.success(m.settings_public_sharing_saved());
		} catch {
			toast.error(m.settings_public_sharing_error());
		}
	}
</script>

<SettingsCard title={m.settings_public_sharing_title()}>
	{#if current !== null}
		{#if isOwner}
			{#key switchKey}
				<SwitchRow
					title={m.settings_public_sharing_switch()}
					hint={m.settings_public_sharing_hint()}
					checked={current.publicShares}
					onchange={flip}
				/>
			{/key}
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
			<SwitchRow
				class="mt-5 border-t border-border pt-5"
				title={m.settings_public_sharing_attribution()}
				hint={m.settings_public_sharing_attribution_hint()}
				checked={current.publicShareAttribution}
				onchange={(on) => save({ publicShareAttribution: on })}
			/>
			<SwitchRow
				class="mt-5 border-t border-border pt-5"
				title={m.settings_public_sharing_sharer()}
				hint={m.settings_public_sharing_sharer_hint()}
				checked={current.publicShareSharer}
				onchange={(on) => save({ publicShareSharer: on })}
			/>
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
</SettingsCard>

<ConfirmDialog
	bind:open={() => confirmOpen, setConfirmOpen}
	title={m.settings_public_sharing_confirm_title()}
	text={m.settings_public_sharing_confirm_text()}
	confirmLabel={m.settings_public_sharing_confirm()}
	onconfirm={() => save({ publicShares: true })}
/>
