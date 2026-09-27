<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import {
		getSettings,
		updateSettings,
		type PreviewLifetime,
		type Settings
	} from '$lib/api/settings';
	import { session } from '$lib/auth.svelte';
	import SegmentedControl from '$lib/components/ui/SegmentedControl.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import { m } from '$lib/paraglide/messages';

	let { ownerName }: { ownerName: string } = $props();

	const isOwner = $derived(session.user?.role === 'superadmin');
	let current = $state<Pick<Settings, 'linkPreviews' | 'linkPreviewMinutes'> | null>(null);

	const lifetimes: Record<PreviewLifetime, () => string> = {
		15: m.settings_link_previews_15,
		60: m.settings_link_previews_60,
		1440: m.settings_link_previews_1440
	};
	const options = ([15, 60, 1440] as PreviewLifetime[]).map((value) => ({
		value: String(value) as `${PreviewLifetime}`,
		label: lifetimes[value]()
	}));

	onMount(async () => {
		try {
			const s = await getSettings();
			current = { linkPreviews: s.linkPreviews, linkPreviewMinutes: s.linkPreviewMinutes };
		} catch {
			current = null;
		}
	});

	// Applies on change, like the editing switch; the card shows the server's
	// answer, so a failed write snaps back.
	async function save(patch: Partial<Settings>) {
		const before = current;
		if (current) {
			current = { ...current, ...patch };
		}
		try {
			const s = await updateSettings(patch);
			current = { linkPreviews: s.linkPreviews, linkPreviewMinutes: s.linkPreviewMinutes };
			toast.success(m.settings_link_previews_saved());
		} catch {
			current = before;
			toast.error(m.settings_link_previews_error());
		}
	}
</script>

<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-link-previews">
	<h2 id="settings-link-previews" class="mb-4 font-display text-heading font-medium">
		{m.settings_link_previews_title()}
	</h2>
	{#if current !== null}
		{#if isOwner}
			<div class="flex items-start justify-between gap-4">
				<div class="min-w-0">
					<p class="text-body font-semibold">{m.settings_link_previews_switch()}</p>
					<p class="mt-1 max-w-[60ch] text-caption text-text-muted">
						{m.settings_link_previews_hint()}
					</p>
				</div>
				<Switch
					checked={current.linkPreviews}
					label={m.settings_link_previews_switch()}
					onchange={(on) => save({ linkPreviews: on })}
				/>
			</div>
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
</section>
