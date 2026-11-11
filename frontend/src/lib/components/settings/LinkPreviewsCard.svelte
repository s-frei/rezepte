<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { updateSettings, type PreviewLifetime, type Settings } from '#lib/api/settings.js';
	import { session } from '#lib/auth.svelte.js';
	import SegmentedControl from '#lib/components/ui/SegmentedControl.svelte';
	import { optimistic } from '#lib/optimistic.js';
	import { m } from '#lib/paraglide/messages.js';
	import SettingsCard from './SettingsCard.svelte';
	import SwitchRow from './SwitchRow.svelte';

	let { ownerName, settings }: { ownerName: string; settings: Settings | null } = $props();

	const isOwner = $derived(session.user?.role === 'superadmin');
	const pick = (s: Settings) => ({
		linkPreviews: s.linkPreviews,
		linkPreviewMinutes: s.linkPreviewMinutes
	});
	let current = $derived(settings && pick(settings));

	const lifetimes: Record<PreviewLifetime, () => string> = {
		15: m.settings_link_previews_15,
		60: m.settings_link_previews_60,
		1440: m.settings_link_previews_1440
	};
	const options = ([15, 60, 1440] as PreviewLifetime[]).map((value) => ({
		value: String(value) as `${PreviewLifetime}`,
		label: lifetimes[value]()
	}));

	// Applies on change, like the editing switch; the card ends on the value
	// the server holds, so a failed write snaps back (see `optimistic`).
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
			if (latest) toast.success(m.settings_link_previews_saved());
		} catch {
			toast.error(m.settings_link_previews_error());
		}
	}
</script>

<SettingsCard title={m.settings_link_previews_title()}>
	{#if current !== null}
		{#if isOwner}
			<SwitchRow
				title={m.settings_link_previews_switch()}
				hint={m.settings_link_previews_hint()}
				checked={current.linkPreviews}
				onchange={(on) => save({ linkPreviews: on })}
			/>
			{#if current.linkPreviews}
				<p class="mt-5 mb-3 text-body-sm text-text-muted">{m.settings_link_previews_lifetime()}</p>
				<SegmentedControl
					value={String(current.linkPreviewMinutes) as `${PreviewLifetime}`}
					{options}
					label={m.settings_link_previews_lifetime()}
					onchange={(next) => save({ linkPreviewMinutes: Number(next) as PreviewLifetime })}
				/>
			{/if}
		{:else if ownerName}
			<!-- Waits for the members list: without it the sentence has no one to name. -->
			<p class="max-w-[60ch] text-body-sm text-text-muted">
				{current.linkPreviews
					? m.settings_link_previews_state_on({
							lifetime: lifetimes[current.linkPreviewMinutes](),
							owner: ownerName
						})
					: m.settings_link_previews_state_off({ owner: ownerName })}
			</p>
		{/if}
	{/if}
</SettingsCard>
