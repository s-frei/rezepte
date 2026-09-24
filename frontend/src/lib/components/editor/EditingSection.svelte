<script lang="ts">
	import { onMount } from 'svelte';
	import type { EditPolicy } from '$lib/api/recipes';
	import { getSettings } from '$lib/api/settings';
	import SegmentedControl from '$lib/components/ui/SegmentedControl.svelte';
	import { m } from '$lib/paraglide/messages';
	import { policyHint } from '$lib/recipe/access';

	let {
		policy = $bindable(),
		canChange,
		isAuthor,
		authorName
	}: {
		policy: EditPolicy;
		/** False for a member editing someone else's recipe (open, or Default while the household is open). */
		canChange: boolean;
		/** Whether the person editing wrote the recipe; the hints say "you" to them and name the author to anyone else. */
		isAuthor: boolean;
		authorName: string;
	} = $props();

	// What "Default" means right now. Unknown until loaded, and on a failed
	// load: the hint then says only that it follows the household setting.
	let lockedByDefault = $state<boolean | null>(null);

	onMount(async () => {
		try {
			lockedByDefault = (await getSettings()).recipesLockedByDefault;
		} catch {
			lockedByDefault = null;
		}
	});

	const options: { value: EditPolicy; label: string }[] = [
		{ value: 'default', label: m.editor_policy_default() },
		{ value: 'open', label: m.editor_policy_open() },
		{ value: 'locked', label: m.editor_policy_locked() }
	];

	const hint = $derived(policyHint(policy, lockedByDefault, { isAuthor, name: authorName }));
</script>

<!--
	A fact, not a field, for someone who may not change it. Such a person only
	reaches the editor while the recipe is editable by everyone: it is either
	open, and the sentence names who opened it, or on Default while the
	household is open, and the Default hint says so. Locked cannot occur here.
-->
{#if canChange}
	<SegmentedControl bind:value={policy} {options} label={m.editor_policy_label()} />
	<p class="mt-3 max-w-[60ch] text-caption text-text-muted" aria-live="polite">{hint}</p>
{:else}
	<p class="max-w-[60ch] text-body-sm text-text-muted">
		{policy === 'open' ? m.editor_policy_readonly_open({ author: authorName }) : hint}
	</p>
{/if}
