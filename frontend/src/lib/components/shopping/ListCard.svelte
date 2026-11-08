<script lang="ts">
	import type { ShoppingSummary } from '#lib/api/shopping.js';
	import { session } from '#lib/auth.svelte.js';
	import PeopleStack from './PeopleStack.svelte';
	import { listHref } from '#lib/shopping/href.js';
	import { relativeTime } from '#lib/shopping/relative.js';
	import { m } from '#lib/paraglide/messages.js';
	import ArrowRight from '@lucide/svelte/icons/arrow-right';

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
	// Who changed it last, unless that is me or the owner the card already names.
	const changed = $derived(
		list.updatedBy && list.updatedBy.id !== meId && (mine || list.updatedBy.id !== list.owner.id)
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
	class="group block rounded-2xl bg-surface px-5 py-4.5 shadow-card transition-[translate,scale,box-shadow,background-color] duration-160 ease-out hover:shadow-lift hover:delay-60 hover:duration-420 hover:ease-spring focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.985] motion-safe:hover:-translate-y-1 motion-reduce:transition-none dark:hover:bg-surface-elevated"
>
	<div class="flex items-start justify-between gap-3">
		<h3
			class="min-w-0 font-display text-card font-medium transition-colors group-hover:text-primary"
		>
			{list.name}
		</h3>
		{#if people.length > 1}<PeopleStack {people} />{/if}
	</div>
	<p class="mt-0.5 text-caption text-text-muted">
		{m.lists_card_counts({ open: list.openCount, done: list.doneCount })}
	</p>
	{#if list.recipes.length > 0}
		<ul class="mt-2.5 flex flex-wrap gap-1.5">
			{#each list.recipes as recipe, i (i)}
				<!-- Only the title gives way; the servings stay readable. -->
				<li
					class="flex max-w-full rounded-pill bg-background px-2.5 py-0.5 text-caption font-semibold text-text-muted"
				>
					<span class="min-w-0 truncate">{recipe.title}</span>
					<span class="shrink-0">&nbsp;·&nbsp;{recipe.servings}</span>
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
	<div class="mt-2 flex items-end gap-3 text-caption">
		<p class="min-w-0 text-text-muted">{who} · {changed}</p>
		<!-- A pointer's hint only: the whole card is the link, and touch never shows it. -->
		<span
			aria-hidden="true"
			class="ml-auto hidden shrink-0 -translate-x-1.5 items-center gap-1 font-semibold whitespace-nowrap text-primary opacity-0 transition-[opacity,translate] duration-420 ease-spring group-hover:translate-x-0 group-hover:opacity-100 motion-reduce:transition-none pointer-fine:inline-flex"
		>
			{m.lists_card_open()}<ArrowRight class="size-3.5" />
		</span>
	</div>
</a>
