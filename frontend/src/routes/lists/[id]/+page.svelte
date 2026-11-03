<script lang="ts">
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import Plus from '@lucide/svelte/icons/plus';
	import Share2 from '@lucide/svelte/icons/share-2';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import {
		addFreeItem,
		clearChecked,
		deleteShoppingList,
		getShoppingList,
		leaveList,
		removeEntry,
		removeItems,
		renameShoppingList,
		setDepartment,
		setEntryServings,
		setItemsChecked,
		type Department,
		type ShoppingEntry,
		type ShoppingList
	} from '#lib/api/shopping.js';
	import AddItemField from '#lib/components/shopping/AddItemField.svelte';
	import EntryCover from '#lib/components/shopping/EntryCover.svelte';
	import EntryMenu from '#lib/components/shopping/EntryMenu.svelte';
	import PeopleStack from '#lib/components/shopping/PeopleStack.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import IconButton from '#lib/components/ui/IconButton.svelte';
	import ShareDialog from '#lib/components/shopping/ShareDialog.svelte';
	import ItemSheet from '#lib/components/shopping/ItemSheet.svelte';
	import ListMenu from '#lib/components/shopping/ListMenu.svelte';
	import ListSide from '#lib/components/shopping/ListSide.svelte';
	import ShoppingRow from '#lib/components/shopping/ShoppingRow.svelte';
	import ViewSwitch from '#lib/components/shopping/ViewSwitch.svelte';
	import { session } from '#lib/auth.svelte.js';
	import { copyText } from '#lib/recipe/share.svelte.js';
	import { servingsUnit } from '#lib/recipe/format.js';
	import { departmentLabel } from '#lib/shopping/departments.js';
	import {
		checkedRows,
		groupByDepartment,
		groupByRecipe,
		listText,
		tickedItemRows,
		type Row
	} from '#lib/shopping/rows.js';
	import { ListState } from '#lib/shopping/list-state.svelte.js';
	import { followList } from '#lib/shopping/live.js';
	import { shopHref } from '#lib/shopping/href.js';
	import { listView } from '#lib/shopping/view.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { shell } from '#lib/shell.svelte.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// Starts as the first load; `load` below follows each later one, and every
	// change replaces the list with the server's answer.
	// svelte-ignore state_referenced_locally
	const lst = new ListState(data.list, fail);
	const list = $derived(lst.list);
	$effect.pre(() => lst.load(data.list));
	// Follows the list live: another member's change refetches it.
	// Only the id: a reload of the same list must not reopen the stream (the first open is not a refetch).
	const listId = $derived(data.list.id);
	$effect(() => followList(listId, () => void lst.refresh()));
	const apply = (request: Promise<ShoppingList>, revert?: () => ShoppingList) =>
		lst.apply(request, revert);

	const departments = $derived(groupByDepartment(list));
	const recipes = $derived(groupByRecipe(list));
	// The recipe view shows each item's own tick, so its "Done" holds every ticked item.
	const done = $derived(
		listView.current === 'department' ? checkedRows(list) : tickedItemRows(list)
	);
	// The header counts what the department view shows: its open rows and its done rows,
	// so "n offen · m erledigt" agrees with "Erledigt · m" (the overview card counts the same rows).
	const openRows = $derived(departments.reduce((n, g) => n + g.rows.length, 0));
	const doneRows = $derived(checkedRows(list).length);
	const empty = $derived(list.items.length === 0 && list.entries.length === 0);

	let sheetOpen = $state(false);
	let sheetRow = $state<Row | null>(null);
	const sheetDepartment = $derived<Department>(
		list.items.find((i) => i.id === sheetRow?.itemIds[0])?.department ?? 'other'
	);

	$effect(() => {
		shell.breadcrumb = m.list_breadcrumb({ lists: m.lists_title(), name: list.name });
		return () => (shell.breadcrumb = undefined);
	});

	const owner = $derived(list.owner.id === session.user?.id);
	const people = $derived([list.owner, ...list.members]);
	let shareOpen = $state(false);

	function fail(error: unknown) {
		if (isSignedOut(error)) return;
		// The list was deleted, or the owner removed me: the server no longer shows it to me.
		if (error instanceof ApiError && error.status === 404) {
			toast.error(m.list_gone());
			void goto(resolve('/lists'));
			return;
		}
		toast.error(m.list_action_error());
	}

	function withChecked(ids: string[], checked: boolean): ShoppingList {
		const set = new Set(ids);
		return {
			...list,
			items: list.items.map((i) => (set.has(i.id) ? { ...i, checked } : i))
		};
	}

	// Optimistic: the circle flips at once; a failed request flips it back.
	async function toggle(row: Row) {
		const checked = !row.checked;
		lst.list = withChecked(row.itemIds, checked);
		await apply(setItemsChecked(list.id, row.itemIds, checked), () =>
			withChecked(row.itemIds, !checked)
		);
	}

	function openSheet(row: Row) {
		sheetRow = row;
		sheetOpen = true;
	}

	async function pickDepartment(department: Department) {
		const row = sheetRow;
		if (!row) return;
		const ids = new Set(row.itemIds);
		lst.list = {
			...list,
			items: list.items.map((i) => (ids.has(i.id) ? { ...i, department } : i))
		};
		// Taken as a change of its own, so a tick sent meanwhile outranks the reload.
		const mine = lst.begin();
		try {
			await setDepartment(row.name, department);
		} catch (error) {
			fail(error);
		}
		// The override reaches every item of that name, on this list too; a failed
		// one is undone by the server's state.
		if (lst.isLatest(mine)) await apply(getShoppingList(list.id));
		else await lst.refresh();
	}

	async function removeSheetRow() {
		if (!sheetRow) return;
		sheetOpen = false;
		await apply(removeItems(list.id, sheetRow.itemIds));
	}

	async function deleteList() {
		try {
			await deleteShoppingList(list.id);
			await goto(resolve('/lists'));
		} catch (error) {
			fail(error);
		}
	}

	async function leave() {
		try {
			await leaveList(list.id);
			toast.success(m.list_left());
			await goto(resolve('/lists'));
		} catch (error) {
			fail(error);
		}
	}

	const changeServings = (entry: ShoppingEntry, servings: number) =>
		apply(setEntryServings(list.id, entry.id, servings));
	const dropEntry = (entry: ShoppingEntry) => apply(removeEntry(list.id, entry.id));
</script>

<svelte:head><title>{list.name} · {m.app_name()}</title></svelte:head>

{#snippet shop(className: string)}
	<Button href={shopHref(list.id)} class="shrink-0 {className}">
		<ShoppingBasket class="size-4" aria-hidden="true" />
		{m.list_shop()}
	</Button>
{/snippet}

<div class="pt-6 pb-8 md:pt-4">
	<!-- Phone: back and ⋯ above the title; desktop: ⋯ beside it. One menu either way. -->
	<header class="mb-4 grid grid-cols-[minmax(0,1fr)_auto] items-end gap-x-4">
		<a
			href={resolve('/lists')}
			aria-label={m.detail_back()}
			class="inline-flex size-10 items-center justify-center justify-self-start rounded-full border border-border bg-surface hover-tint text-text transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:hidden"
		>
			<ArrowLeft class="size-5" aria-hidden="true" />
		</a>
		<div class="col-span-2 row-start-2 mt-5 min-w-0 md:col-span-1 md:row-start-1 md:mt-0">
			<h1 class="font-display text-display-md font-medium wrap-break-word md:text-display-lg">
				{list.name}
			</h1>
			<!-- Phone: "Shop" right of the counts; desktop: first in the header's buttons. -->
			<div class="mt-1.5 flex items-center justify-between gap-3">
				<p class="text-caption text-text-muted">
					{m.list_counts({ open: openRows, done: doneRows })}
				</p>
				{@render shop('md:hidden')}
			</div>
		</div>
		<div class="col-start-2 row-start-1 flex items-center gap-2 md:mb-1">
			{@render shop('max-md:hidden')}
			{#if people.length > 1}<PeopleStack {people} />{/if}
			{#if owner}
				<IconButton label={m.list_share()} onclick={() => (shareOpen = true)} class="md:hidden">
					<Share2 class="size-[18px]" aria-hidden="true" />
				</IconButton>
				<Button variant="secondary" onclick={() => (shareOpen = true)} class="max-md:hidden">
					<Share2 class="size-4" aria-hidden="true" />
					{m.list_share()}
				</Button>
			{/if}
			<ListMenu
				name={list.name}
				{owner}
				onshare={() => (shareOpen = true)}
				onleave={leave}
				onrename={(name) => apply(renameShoppingList(list.id, name))}
				oncopy={() => copyText(listText(list), m.list_copied())}
				ondelete={deleteList}
			/>
		</div>
	</header>

	<div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px] lg:items-start">
		<!--
			View switch A3 with one instance: on a desktop it sits right of the add
			field in the first row; on a phone it moves into the list card's first
			row, over the card's background (an empty grid cell spanning the
			switch's row and the content's), with a dashed rule below it.
		-->
		<div class="grid min-w-0 grid-cols-1 md:grid-cols-[minmax(0,1fr)_auto] md:gap-x-3">
			<AddItemField
				class="md:col-start-1 md:row-start-1"
				onadd={(text) => apply(addFreeItem(list.id, text))}
			/>
			<div
				aria-hidden="true"
				class="col-span-full row-start-2 row-end-4 mt-4 rounded-2xl bg-surface shadow-card md:row-end-3"
			></div>
			<div
				class="col-start-1 row-start-2 mx-4.5 mt-4 border-b border-dashed border-border pt-3.5 pb-3.5 md:col-start-2 md:row-start-1 md:m-0 md:border-0 md:p-0"
			>
				<ViewSwitch />
			</div>
			<div
				class="col-span-full row-start-3 px-4.5 pb-3 md:row-start-2 md:mt-4 md:px-6 md:pt-1.5 md:pb-4"
			>
				{#if empty}
					<div class="py-10 text-center">
						<p class="font-display text-heading font-medium">{m.list_empty_title()}</p>
						<p class="mx-auto mt-1.5 max-w-sm text-body text-text-muted">{m.list_empty_text()}</p>
					</div>
				{:else if listView.current === 'department'}
					{#each departments as group (group.department)}
						<section>
							<h2
								class="mt-4.5 mb-0.5 flex items-baseline justify-between gap-3 font-display text-body-lg font-medium text-primary italic"
							>
								{departmentLabel(group.department)}
								<span class="font-sans text-caption font-semibold text-text-muted not-italic"
									>{group.rows.length}</span
								>
							</h2>
							<ul>
								{#each group.rows as row (row.key)}
									<ShoppingRow {row} onToggle={toggle} onOpen={openSheet} />
								{/each}
							</ul>
						</section>
					{/each}
				{:else}
					{#each recipes as group, i (group.entry?.id ?? 'free')}
						{@const rows = group.rows.filter((r) => !r.checked)}
						{#if group.entry || rows.length > 0}
							<section class={i > 0 ? 'mt-2 border-t border-border' : ''}>
								<div class="flex items-center gap-3 pt-4 pb-1">
									{#if group.entry}
										{@const entry = group.entry}
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
										<EntryMenu
											{entry}
											onservings={(n) => changeServings(entry, n)}
											onremove={() => dropEntry(entry)}
										/>
									{:else}
										<span
											aria-hidden="true"
											class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-accent text-accent-foreground"
										>
											<Plus class="size-[18px]" />
										</span>
										<div class="min-w-0 flex-1">
											<h2 class="font-display text-card font-medium">{m.list_free_items()}</h2>
											<p class="text-caption text-text-muted">{m.list_free_items_hint()}</p>
										</div>
									{/if}
								</div>
								<ul>
									{#each rows as row (row.key)}
										<ShoppingRow {row} bySource onToggle={toggle} onOpen={openSheet} />
									{/each}
								</ul>
							</section>
						{/if}
					{/each}
				{/if}

				{#if done.length > 0}
					<section class="mt-5">
						<div class="flex items-center justify-between gap-2">
							<h2 class="font-display text-body-lg font-medium text-text-muted italic">
								{m.list_done_heading({ count: done.length })}
							</h2>
							<button
								type="button"
								onclick={() => apply(clearChecked(list.id))}
								class="h-8 rounded-pill px-1 text-body-sm font-semibold text-primary transition hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
							>
								{m.list_clear_checked()}
							</button>
						</div>
						<ul>
							{#each done as row (row.key)}
								<ShoppingRow {row} onToggle={toggle} onOpen={openSheet} />
							{/each}
						</ul>
					</section>
				{/if}
			</div>
		</div>

		<aside class="min-w-0 max-lg:hidden">
			<ListSide
				{list}
				onservings={changeServings}
				onremove={dropEntry}
				onshare={owner ? () => (shareOpen = true) : undefined}
			/>
		</aside>
	</div>
</div>

<ShareDialog bind:open={shareOpen} {list} {apply} />

<ItemSheet
	bind:open={sheetOpen}
	row={sheetRow}
	department={sheetDepartment}
	onpick={pickDepartment}
	onremove={removeSheetRow}
/>
