<script lang="ts">
	import Share2 from '@lucide/svelte/icons/share-2';
	import type { ShoppingEntry, ShoppingList } from '#lib/api/shopping.js';
	import { servingsUnit } from '#lib/recipe/format.js';
	import { m } from '#lib/paraglide/messages.js';
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

	function itemCount(entry: ShoppingEntry): string {
		const count = list.items.filter((i) => i.entryId === entry.id).length;
		return count === 1 ? m.list_entry_items_one() : m.list_entry_items({ count });
	}
</script>

<div class="grid grid-cols-1 gap-4">
	{#if list.entries.length > 0}
		<section class="rounded-2xl bg-surface px-5 py-4.5 shadow-card">
			<h2 class="mb-1 font-display text-heading font-medium">{m.list_side_recipes()}</h2>
			<ul>
				{#each list.entries as entry (entry.id)}
					<li
						class="flex items-center gap-3 border-b border-dashed border-border py-2.5 last:border-b-0"
					>
						<EntryCover {entry} class="size-11 text-body-lg" />
						<div class="min-w-0 flex-1">
							<p class="text-body font-semibold text-pretty break-words">{entry.title}</p>
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
		<PeopleList {list} />
	</section>
</div>
