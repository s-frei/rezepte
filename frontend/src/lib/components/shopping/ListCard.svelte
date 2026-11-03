<script lang="ts">
	import type { ShoppingSummary } from '#lib/api/shopping.js';
	import { listHref } from '#lib/shopping/href.js';
	import { relativeTime } from '#lib/shopping/relative.js';
	import { m } from '#lib/paraglide/messages.js';

	let { list }: { list: ShoppingSummary } = $props();

	const total = $derived(list.openCount + list.doneCount);
	const percent = $derived(total === 0 ? 0 : Math.round((list.doneCount / total) * 100));
</script>

<a
	href={listHref(list.id)}
	class="block rounded-2xl bg-surface hover-tint px-5 py-4.5 shadow-card focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
>
	<h3 class="font-display text-card font-medium">{list.name}</h3>
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
		{m.lists_card_only_you()} · {m.lists_card_changed({ when: relativeTime(list.updatedAt) })}
	</p>
</a>
