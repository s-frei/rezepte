<script lang="ts">
	import { session } from '#lib/auth.svelte.js';
	import type { ShoppingEntry, ShoppingList } from '#lib/api/shopping.js';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { servingsUnit } from '#lib/recipe/format.js';
	import { m } from '#lib/paraglide/messages.js';
	import EntryCover from './EntryCover.svelte';
	import EntryMenu from './EntryMenu.svelte';

	let {
		list,
		onservings,
		onremove
	}: {
		list: ShoppingList;
		onservings: (entry: ShoppingEntry, servings: number) => void;
		onremove: (entry: ShoppingEntry) => void;
	} = $props();

	function itemCount(entry: ShoppingEntry): string {
		const count = list.items.filter((i) => i.entryId === entry.id).length;
		return count === 1 ? m.list_entry_items_one() : m.list_entry_items({ count });
	}
</script>

<!-- Phase 1 has no sharing: "Shared with" names only the creator, the one who sees the list. -->
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
							<p class="truncate text-body font-semibold">{entry.title}</p>
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
	{#if session.user}
		<section class="rounded-2xl bg-surface px-5 py-4.5 shadow-card">
			<h2 class="mb-1 font-display text-heading font-medium">{m.list_side_people()}</h2>
			<div class="flex items-center gap-3 py-2">
				<PersonMark person={session.user} size="sm" />
				<div class="min-w-0">
					<p class="truncate text-body font-semibold">{session.user.displayName}</p>
					<p class="text-caption text-text-muted">{m.list_side_you_created()}</p>
				</div>
			</div>
		</section>
	{/if}
</div>
