<script lang="ts">
	import type { ShoppingSummary } from '#lib/api/shopping.js';
	import { session } from '#lib/auth.svelte.js';
	import PeopleStack from './PeopleStack.svelte';
	import { listHref } from '#lib/shopping/href.js';
	import { relativeTime } from '#lib/shopping/relative.js';
	import { m } from '#lib/paraglide/messages.js';

	let { list }: { list: ShoppingSummary } = $props();

	const meId = $derived(session.user?.id);
	const mine = $derived(list.owner.id === meId);
	const people = $derived([list.owner, ...list.members]);
	// Members by name, "you" where I am one of them.
	const names = $derived(
		list.members.map((p) => (p.id === meId ? m.lists_you() : p.displayName)).join(', ')
	);
	const who = $derived(
		mine
			? list.members.length === 0
				? m.lists_card_only_you()
				: m.lists_card_shared_with({ names })
			: m.lists_card_from({ owner: list.owner.displayName, names })
	);
	const changed = $derived(
		list.updatedBy && list.updatedBy.id !== meId
			? m.lists_card_changed_by({
					name: list.updatedBy.displayName,
					when: relativeTime(list.updatedAt)
				})
			: m.lists_card_changed({ when: relativeTime(list.updatedAt) })
	);
	const total = $derived(list.openCount + list.doneCount);
	const percent = $derived(total === 0 ? 0 : Math.round((list.doneCount / total) * 100));
</script>

<a
	href={listHref(list.id)}
	class="block rounded-2xl bg-surface hover-tint px-5 py-4.5 shadow-card focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
>
	<div class="flex items-start justify-between gap-3">
		<h3 class="min-w-0 font-display text-card font-medium">{list.name}</h3>
		{#if people.length > 1}<PeopleStack {people} />{/if}
	</div>
	<p class="mt-0.5 text-caption text-text-muted">
		{m.lists_card_counts({ open: list.openCount, done: list.doneCount })}
	</p>
	{#if list.recipes.length > 0}
		<ul class="mt-2.5 flex flex-wrap gap-1.5">
			{#each list.recipes as recipe, i (i)}
				<li
					class="max-w-full truncate rounded-pill bg-background px-2.5 py-0.5 text-caption font-semibold text-text-muted"
				>
					{recipe.title} · {recipe.servings}
				</li>
			{/each}
		</ul>
	{/if}
	<div
		role="progressbar"
		aria-label={m.lists_card_counts({ open: list.openCount, done: list.doneCount })}
		aria-valuemin={0}
		aria-valuemax={100}
		aria-valuenow={percent}
		class="mt-3.5 h-1.5 overflow-hidden rounded-pill bg-background"
	>
		<div class="h-full rounded-pill bg-primary" style:width="{percent}%"></div>
	</div>
	<p class="mt-2 text-caption text-text-muted">
		{who} · {changed}
	</p>
</a>
