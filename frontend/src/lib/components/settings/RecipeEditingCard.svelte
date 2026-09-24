<script lang="ts">
	import { onMount } from 'svelte';
	import ArrowUpRight from 'lucide-svelte/icons/arrow-up-right';
	import { toast } from 'svelte-sonner';
	import { getSettings, setRecipesLockedByDefault } from '$lib/api/settings';
	import { session } from '$lib/auth.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import { USER_GUIDE_URL } from '$lib/docs';
	import { m } from '$lib/paraglide/messages';

	let { ownerName }: { ownerName: string } = $props();

	const isOwner = $derived(session.user?.role === 'superadmin');
	let locked = $state<boolean | null>(null);

	onMount(async () => {
		try {
			locked = (await getSettings()).recipesLockedByDefault;
		} catch {
			locked = null;
		}
	});

	// Applies on flip, like theme and language; the switch shows the server's
	// answer, so a failed write snaps back.
	async function change(next: boolean) {
		const before = locked;
		locked = next;
		try {
			locked = (await setRecipesLockedByDefault(next)).recipesLockedByDefault;
			toast.success(
				locked ? m.settings_recipe_editing_saved_on() : m.settings_recipe_editing_saved_off()
			);
		} catch {
			locked = before;
			toast.error(m.settings_recipe_editing_error());
		}
	}
</script>

<section class="rounded-2xl bg-surface p-6 md:p-7" aria-labelledby="settings-recipe-editing">
	<h2 id="settings-recipe-editing" class="mb-4 font-display text-heading font-medium">
		{m.settings_recipe_editing_title()}
	</h2>
	{#if locked !== null}
		{#if isOwner}
			<div class="flex items-start justify-between gap-4">
				<div class="min-w-0">
					<p class="text-body font-semibold">{m.settings_recipe_editing_switch()}</p>
					<p class="mt-1 max-w-[60ch] text-caption text-text-muted">
						{m.settings_recipe_editing_hint()}
					</p>
				</div>
				<Switch checked={locked} label={m.settings_recipe_editing_switch()} onchange={change} />
			</div>
		{:else if ownerName}
			<!-- Waits for the members list: without it the sentence has no one to name. -->
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
		class="mt-3 inline-flex h-10 items-center gap-1.5 rounded-pill text-body-sm text-text-muted underline decoration-border underline-offset-4 transition hover:text-text hover:decoration-text-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
	>
		{m.settings_recipe_editing_guide()}
		<ArrowUpRight aria-hidden="true" class="size-4" />
		<span class="sr-only">({m.settings_help_new_tab()})</span>
	</a>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
</section>
