<script lang="ts">
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import { toast } from 'svelte-sonner';
	import { setRecipesLockedByDefault, type Settings } from '#lib/api/settings.js';
	import { session } from '#lib/auth.svelte.js';
	import { USER_GUIDE_URL } from '#lib/docs.js';
	import { optimistic } from '#lib/optimistic.js';
	import { m } from '#lib/paraglide/messages.js';
	import SettingsCard from './SettingsCard.svelte';
	import SwitchRow from './SwitchRow.svelte';

	let { ownerName, settings }: { ownerName: string; settings: Settings | null } = $props();

	const isOwner = $derived(session.user?.role === 'superadmin');
	let locked = $derived(settings?.recipesLockedByDefault ?? null);

	// Applies on flip, like theme and language; the switch ends on the value
	// the server holds, so a failed write snaps back (see `optimistic`).
	const save = optimistic(
		() => locked,
		(value) => (locked = value)
	);
	async function change(next: boolean) {
		try {
			const { answer, latest } = await save(
				next,
				() => setRecipesLockedByDefault(next),
				(s) => s.recipesLockedByDefault
			);
			if (!latest) return;
			toast.success(
				answer.recipesLockedByDefault
					? m.settings_recipe_editing_saved_on()
					: m.settings_recipe_editing_saved_off()
			);
		} catch {
			toast.error(m.settings_recipe_editing_error());
		}
	}
</script>

<SettingsCard title={m.settings_recipe_editing_title()}>
	{#if locked !== null}
		{#if isOwner}
			<SwitchRow
				title={m.settings_recipe_editing_switch()}
				hint={m.settings_recipe_editing_hint()}
				checked={locked}
				onchange={change}
			/>
		{:else if ownerName}
			<!-- Waits for the people list: without it the sentence has no one to name. -->
			<p class="max-w-[60ch] text-body-sm text-text-muted">
				{locked
					? m.settings_recipe_editing_state_locked({ owner: ownerName })
					: m.settings_recipe_editing_state_open({ owner: ownerName })}
			</p>
		{/if}
	{/if}
	<!--
		The guide lives on GitHub Pages, outside the SPA router, so `resolve()`
		does not apply and the navigation rule is lifted for this anchor only.
		Styled like the API card's secondary link, with the arrow its docs
		button uses for "leaves the app".
	-->
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<a
		href={`${USER_GUIDE_URL}editing-rights/`}
		target="_blank"
		rel="noopener external"
		class="mt-3 inline-flex min-h-10 max-w-full items-center gap-1.5 rounded-pill text-body-sm text-text-muted underline decoration-border underline-offset-4 transition hover:text-text hover:decoration-text-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
	>
		{m.settings_recipe_editing_guide()}
		<ArrowUpRight aria-hidden="true" class="size-4" />
		<span class="sr-only">({m.settings_help_new_tab()})</span>
	</a>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
</SettingsCard>
