<script lang="ts">
	import BookOpen from '@lucide/svelte/icons/book-open';
	import Plus from '@lucide/svelte/icons/plus';
	import { getLocalTimeZone, today } from '@internationalized/date';
	import type { Snippet } from 'svelte';
	import type { ShoppingEntry } from '#lib/api/shopping.js';
	import { session } from '#lib/auth.svelte.js';
	import EntryCover from '#lib/components/shopping/EntryCover.svelte';
	import LinkStamp from '#lib/components/shopping/LinkStamp.svelte';
	import { mondayOf } from '#lib/mealplan/week.js';
	import { planStamp } from '#lib/shopping/plan-ref.js';
	import ShoppingRow from '#lib/components/shopping/ShoppingRow.svelte';
	import { servingsUnit } from '#lib/recipe/format.js';
	import { departmentLabel } from '#lib/shopping/departments.js';
	import type { groupByDepartment, groupByRecipe, Row } from '#lib/shopping/rows.js';
	import { listView } from '#lib/shopping/view.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * The open rows of a list, by department or by recipe (`listView`), on the
	 * list page and in shopping mode (`shop`: larger rows, no item sheet, no
	 * entry menu, and a recipe whose rows are all ticked drops out). On the
	 * list page (`stamps`) a planned block wears its plan's stamp, which
	 * names the day, and one added by hand says so.
	 */
	let {
		departments,
		recipes,
		checkers,
		shop = false,
		flashId = null,
		onToggle,
		onOpen,
		entryMenu,
		stamps = false
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
		stamps?: boolean;
	} = $props();

	const thisMonday = mondayOf(today(getLocalTimeZone()).toString());
	/** Only beside planned blocks does "added by hand" tell anything. */
	const anyPlanned = $derived(recipes.some((g) => g.entry?.planId));
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
			<!-- A stamp's jump lights the block (`data-lit`) on a layer behind it
			     that reaches past its sides, so the rows keep room to the edge. -->
			<section
				data-entry-block={group.entry?.id}
				class="relative isolate scroll-mt-4 before:absolute before:-inset-x-2.5 before:inset-y-0 before:-z-10 before:rounded-lg data-lit:before:motion-safe:animate-flash md:scroll-mt-24 {!shop &&
				i > 0
					? 'mt-2 border-t border-border'
					: ''}"
			>
				<!-- A recipe's head: the ⋯ beside the title at the top, so the plan
				     stamp and the servings below run under it, the row's full width. -->
				<div
					class="{group.entry
						? 'grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-x-3'
						: 'flex items-center gap-3'} pt-4 pb-1 {shop ? 'mt-2' : ''}"
				>
					{#if group.entry}
						<EntryCover entry={group.entry} class="row-span-2 size-10 text-body-lg" />
						{@const plan = stamps ? planStamp(group.entry, session.user?.id, thisMonday) : null}
						<h2
							tabindex="-1"
							class="col-start-2 row-start-1 self-end font-display text-card font-medium text-pretty hyphens-auto outline-none"
						>
							<!-- "Mi ·" stays one piece; with a stamp, the stamp names the day. -->
							{#if group.day && !stamps}<span class="text-text-muted">{group.day}&nbsp;·</span>{/if}
							{group.entry.title}
						</h2>
						<div class="col-start-3 row-start-1 self-start">
							{@render entryMenu?.(group.entry)}
						</div>
						<!-- A container: the stamp's date gives way before its plan's name. -->
						<div class="@container col-span-2 col-start-2 row-start-2 min-w-0">
							<div
								class="flex flex-wrap items-center gap-x-2.5 gap-y-1 {stamps && anyPlanned
									? 'mt-1'
									: ''}"
							>
								{#if plan}
									<LinkStamp
										kind="plan"
										label={plan.label}
										day={group.entry.planDay}
										href={plan.href}
										quiet={!plan.member}
									/>
								{:else if stamps && anyPlanned}
									<!-- Added by hand: said, not left to a missing stamp. -->
									<span
										class="inline-flex items-center gap-1 text-caption whitespace-nowrap text-text-muted"
									>
										<BookOpen class="size-3.5 shrink-0" aria-hidden="true" />
										{m.link_single()}
									</span>
								{/if}
								<span class="text-caption whitespace-nowrap text-text-muted">
									{group.entry.servings}
									{servingsUnit(group.entry.servings)}
								</span>
							</div>
						</div>
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
