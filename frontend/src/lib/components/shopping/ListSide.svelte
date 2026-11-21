<script lang="ts">
	import BookOpen from '@lucide/svelte/icons/book-open';
	import CalendarDays from '@lucide/svelte/icons/calendar-days';
	import Share2 from '@lucide/svelte/icons/share-2';
	import { getLocalTimeZone, today } from '@internationalized/date';
	import { session } from '#lib/auth.svelte.js';
	import { mondayOf, weekRange } from '#lib/mealplan/week.js';
	import { groupByPlan } from '#lib/shopping/plan-ref.js';
	import type { ShoppingEntry, ShoppingList } from '#lib/api/shopping.js';
	import { servingsUnit } from '#lib/recipe/format.js';
	import { plannedDay } from '#lib/shopping/rows.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import EntryCover from './EntryCover.svelte';
	import EntryMenu from './EntryMenu.svelte';
	import PeopleList from './PeopleList.svelte';

	let {
		list,
		onservings,
		onremove,
		onshare
	}: {
		list: ShoppingList;
		onservings: (entry: ShoppingEntry, servings: number) => void;
		onremove: (entry: ShoppingEntry) => void;
		/** Given for the creator only, who alone may share. */
		onshare?: () => void;
	} = $props();

	/** The recipes under a heading per plan and week ("Haushalt · 16.–22. Nov."), the ones added by hand last. */
	const groups = $derived(
		groupByPlan(list.entries, session.user?.id, mondayOf(today(getLocalTimeZone()).toString()))
	);

	function itemCount(entry: ShoppingEntry): string {
		const count = list.items.filter((i) => i.entryId === entry.id).length;
		return count === 1 ? m.list_entry_items_one() : m.list_entry_items({ count });
	}
</script>

<div class="grid grid-cols-1 gap-4">
	{#if list.entries.length > 0}
		<section class="rounded-2xl bg-surface px-5 py-4.5 shadow-card">
			<h2 class="mb-1 font-display text-heading font-medium">{m.list_side_recipes()}</h2>
			{#each groups as group, i (group.key)}
				<!-- A list without a plan's recipes needs no headings. -->
				<h3
					hidden={!group.plan && groups.length === 1}
					class="flex items-center gap-1.5 pt-3 pb-0.5 text-caption font-semibold {i > 0
						? 'mt-1 border-t border-border'
						: ''} {group.plan?.member ? 'text-plan-foreground' : 'text-text-muted'}"
				>
					{#if group.plan}
						{@const week = group.monday ? weekRange(group.monday, getLocale(), 'short') : ''}
						{#snippet head()}
							<CalendarDays class="size-3.5 shrink-0" aria-hidden="true" />
							<!-- The date wraps out of the clipped line when it does not fit: the name is never cut for it. -->
							<span class="flex h-[1lh] min-w-0 flex-wrap overflow-hidden"
								><span class="max-w-full truncate">{group.plan?.label}</span>{#if week}<span
										class="shrink-0">&nbsp;·&nbsp;{week}</span
									>{/if}</span
							>
						{/snippet}
						{#if group.plan.href}
							<!-- The way into the plan, at the group's first day. -->
							<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- resolved in planStamp -->
							<a
								href={group.plan.href}
								aria-label={[
									m.link_aria({ kind: m.link_kind_plan(), name: group.plan.label }),
									week
								]
									.filter(Boolean)
									.join(', ')}
								class="flex min-w-0 items-center gap-1.5 rounded-sm hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
								>{@render head()}</a
							>
						{:else}
							{@render head()}
						{/if}
					{:else}
						<BookOpen class="size-3.5 shrink-0" aria-hidden="true" />
						<span class="min-w-0 truncate">{m.link_single()}</span>
					{/if}
				</h3>
				<ul>
					{#each group.items as entry (entry.id)}
						{@const day = plannedDay(entry)}
						<li
							class="flex items-center gap-3 border-b border-dashed border-border py-2.5 last:border-b-0"
						>
							<EntryCover {entry} class="size-11 text-body-lg" />
							<div class="min-w-0 flex-1">
								<p class="text-body font-semibold text-pretty break-words">
									{#if day}<span class="font-normal text-text-muted">{day}&nbsp;·</span>{/if}
									{entry.title}
								</p>
								<p class="truncate text-caption text-text-muted">
									{entry.servings}
									{servingsUnit(entry.servings)} · {itemCount(entry)}
								</p>
							</div>
							<EntryMenu
								{entry}
								onservings={(n) => onservings(entry, n)}
								onremove={() => onremove(entry)}
							/>
						</li>
					{/each}
				</ul>
			{/each}
		</section>
	{/if}
	<section class="rounded-2xl bg-surface px-5 py-4.5 shadow-card">
		<div class="mb-1 flex items-center justify-between gap-3">
			<h2 class="font-display text-heading font-medium">{m.list_side_people()}</h2>
			{#if onshare}
				<button
					type="button"
					onclick={onshare}
					class="inline-flex h-8 items-center gap-1.5 rounded-pill text-body-sm font-semibold text-primary transition hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<Share2 class="size-4" aria-hidden="true" />
					{m.list_share()}
				</button>
			{/if}
		</div>
		<PeopleList owner={list.owner} members={list.members} />
	</section>
</div>
