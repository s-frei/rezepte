<script lang="ts">
	import Plus from '@lucide/svelte/icons/plus';
	import type { Snippet } from 'svelte';
	import type { ShoppingEntry } from '#lib/api/shopping.js';
	import EntryCover from '#lib/components/shopping/EntryCover.svelte';
	import ShoppingRow from '#lib/components/shopping/ShoppingRow.svelte';
	import { servingsUnit } from '#lib/recipe/format.js';
	import { departmentLabel } from '#lib/shopping/departments.js';
	import type { groupByDepartment, groupByRecipe, Row } from '#lib/shopping/rows.js';
	import { listView } from '#lib/shopping/view.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * The open rows of a list, by department or by recipe (`listView`), on the
	 * list page and in shopping mode (`shop`: larger rows, no item sheet, no
	 * entry menu, and a recipe whose rows are all ticked drops out).
	 */
	let {
		departments,
		recipes,
		checkers,
		shop = false,
		flashId = null,
		onToggle,
		onOpen,
		entryMenu
	}: {
		departments: ReturnType<typeof groupByDepartment>;
		recipes: ReturnType<typeof groupByRecipe>;
		checkers: boolean;
		shop?: boolean;
		/** The item just added: its row lights up once. */
		flashId?: string | null;
		onToggle: (row: Row) => void;
		onOpen?: (row: Row) => void;
		/** The ⋯ beside a recipe's title. */
		entryMenu?: Snippet<[ShoppingEntry]>;
	} = $props();
</script>

{#snippet item(row: Row, bySource = false)}
	<ShoppingRow
		{checkers}
		{row}
		{bySource}
		{shop}
		flash={!!flashId && row.itemIds.includes(flashId)}
		{onToggle}
		{onOpen}
	/>
{/snippet}

{#if listView.current === 'department'}
	{#each departments as group (group.department)}
		<section>
			<h2
				class="mt-4.5 mb-0.5 flex items-baseline justify-between gap-3 font-display {shop
					? 'text-heading'
					: 'text-body-lg'} font-medium text-primary italic"
			>
				{departmentLabel(group.department)}
				<span
					class="font-sans {shop
						? 'text-body-sm'
						: 'text-caption'} font-semibold text-text-muted not-italic">{group.rows.length}</span
				>
			</h2>
			<ul>
				{#each group.rows as row (row.key)}
					{@render item(row)}
				{/each}
			</ul>
		</section>
	{/each}
{:else}
	{#each recipes as group, i (group.entry?.id ?? 'free')}
		{@const rows = group.rows.filter((r) => !r.checked)}
		{#if (group.entry && !shop) || rows.length > 0}
			<section class={!shop && i > 0 ? 'mt-2 border-t border-border' : ''}>
				<div class="flex items-center gap-3 pt-4 pb-1 {shop ? 'mt-2' : ''}">
					{#if group.entry}
						<EntryCover entry={group.entry} class="size-10 text-body-lg" />
						<div class="min-w-0 flex-1">
							<h2 class="font-display text-card font-medium text-pretty">
								{group.entry.title}
							</h2>
							<p class="text-caption text-text-muted">
								{group.entry.servings}
								{servingsUnit(group.entry.servings)}
							</p>
						</div>
						{@render entryMenu?.(group.entry)}
					{:else}
						<span
							aria-hidden="true"
							class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-accent text-accent-foreground"
						>
							<Plus class="size-[18px]" />
						</span>
						<div class="min-w-0 flex-1">
							<h2 class="font-display text-card font-medium">{m.list_free_items()}</h2>
							{#if !shop}
								<p class="text-caption text-text-muted">{m.list_free_items_hint()}</p>
							{/if}
						</div>
					{/if}
				</div>
				<ul>
					{#each rows as row (row.key)}
						{@render item(row, true)}
					{/each}
				</ul>
			</section>
		{/if}
	{/each}
{/if}
