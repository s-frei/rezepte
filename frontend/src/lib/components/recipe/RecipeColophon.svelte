<script lang="ts">
	import Globe from '@lucide/svelte/icons/globe';
	import Lock from '@lucide/svelte/icons/lock';
	import type { Recipe } from '$lib/api/recipes';
	import type { PublicShare } from '$lib/api/shares';
	import { session } from '$lib/auth.svelte';
	import { m } from '$lib/paraglide/messages';
	import { lockNotice } from '$lib/recipe/access';
	import { hasBeenEdited } from '$lib/recipe/authorship';
	import { formatDate } from '$lib/recipe/format';

	let {
		recipe,
		share = null,
		onmanageshare
	}: {
		recipe: Pick<Recipe, 'createdBy' | 'createdAt' | 'updatedBy' | 'updatedAt' | 'locked'>;
		/** The viewer's own public link to this recipe, if any: its line says
		 * how long it runs, or why it does not, and opens the dialog. */
		share?: PublicShare | null;
		onmanageshare?: () => void;
	} = $props();

	const edited = $derived(hasBeenEdited(recipe));
	const notice = $derived(lockNotice(recipe, session.user?.id));
	const shareLine = $derived.by(() => {
		if (!share) return '';
		if (share.status === 'paused') return m.detail_public_share_paused();
		if (share.status === 'limited') return m.detail_public_share_limited();
		return share.expiresAt
			? m.detail_public_share_until({ date: formatDate(share.expiresAt) })
			: m.detail_public_share_permanent();
	});
</script>

<footer class="mt-10 border-t border-border pt-6 text-caption text-text-muted">
	<p>
		{m.detail_created_by({
			user: recipe.createdBy.displayName,
			date: formatDate(recipe.createdAt)
		})}
	</p>
	{#if edited}
		<p class="mt-1">
			{m.detail_updated_by({
				user: recipe.updatedBy.displayName,
				date: formatDate(recipe.updatedAt)
			})}
		</p>
	{/if}
	{#if notice}
		<p class="mt-1 flex items-center gap-1.5">
			<Lock class="size-3.5 shrink-0" aria-hidden="true" />
			{notice}
		</p>
	{/if}
	{#if shareLine}
		<p class="mt-1 flex flex-wrap items-center gap-x-1.5">
			<Globe class="size-3.5 shrink-0 text-primary" aria-hidden="true" />
			<span class="font-semibold text-text">{shareLine}</span>
			{#if onmanageshare}
				<span aria-hidden="true">·</span>
				<button
					type="button"
					onclick={onmanageshare}
					class="font-semibold text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					{m.detail_public_share_manage()}
				</button>
			{/if}
		</p>
	{/if}
</footer>
