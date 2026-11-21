<script lang="ts">
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import Share2 from '@lucide/svelte/icons/share-2';
	import { getLocalTimeZone, today } from '@internationalized/date';
	import { toast } from 'svelte-sonner';
	import { onDestroy, tick } from 'svelte';
	import { afterNavigate, beforeNavigate, goto, replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import {
		addFreeItem,
		clearChecked,
		deleteShoppingList,
		getShoppingList,
		inviteToList,
		leaveList,
		removeEntry,
		removeListMember,
		removeItems,
		renameShoppingList,
		setDepartment,
		setEntryServings,
		type Department,
		type ShoppingEntry,
		type ShoppingList
	} from '#lib/api/shopping.js';
	import AddItemCard from '#lib/components/shopping/AddItemCard.svelte';
	import EntryMenu from '#lib/components/shopping/EntryMenu.svelte';
	import PeopleSheet from '#lib/components/shopping/PeopleSheet.svelte';
	import PeopleStack from '#lib/components/shopping/PeopleStack.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import IconButton from '#lib/components/ui/IconButton.svelte';
	import ShareButton from '#lib/components/ui/ShareButton.svelte';
	import ShareDialog from '#lib/components/shopping/ShareDialog.svelte';
	import ItemSheet from '#lib/components/shopping/ItemSheet.svelte';
	import ListImportDialog from '#lib/components/shopping/ListImportDialog.svelte';
	import ListGroups from '#lib/components/shopping/ListGroups.svelte';
	import ListMenu from '#lib/components/shopping/ListMenu.svelte';
	import ListPanel from '#lib/components/shopping/ListPanel.svelte';
	import ListSide from '#lib/components/shopping/ListSide.svelte';
	import ShoppingRow from '#lib/components/shopping/ShoppingRow.svelte';
	import StampRow from '#lib/components/shopping/StampRow.svelte';
	import { mondayOf } from '#lib/mealplan/week.js';
	import { uniquePlans } from '#lib/shopping/plan-ref.js';
	import { session } from '#lib/auth.svelte.js';
	import { copyText } from '#lib/recipe/share.svelte.js';
	import { departmentLabel } from '#lib/shopping/departments.js';
	import { listText, listViews, type Row } from '#lib/shopping/rows.js';
	import { failList } from '#lib/shopping/fail.js';
	import { ListState } from '#lib/shopping/list-state.svelte.js';
	import { shopHref } from '#lib/shopping/href.js';
	import { borrowListView, listView } from '#lib/shopping/view.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { shell } from '#lib/shell.svelte.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// Arrived from the draft on /lists/new: the head's actions and "Shop" fade
	// in, the dimmed rest brightens, the caret goes to "What do you need?".
	const created = page.state.created === true;
	const arrive = created ? 'motion-safe:animate-fade-in' : '';

	// Starts as the first load; `connect` follows each later one, and every
	// change replaces the list with the server's answer.
	// svelte-ignore state_referenced_locally
	const lst = new ListState(data.list, failList);
	lst.connect(() => data.list);
	// `shown`: what another device just ticked stays in place for a moment.
	const list = $derived(lst.shown);

	const { departments, recipes, done, openRows, doneRows } = $derived(
		listViews(list, listView.current)
	);
	const empty = $derived(list.items.length === 0 && list.entries.length === 0);
	// Every plan the recipes came from: a link into a plan the reader is in.
	const fromPlans = $derived(
		uniquePlans(list.entries, session.user?.id, mondayOf(today(getLocalTimeZone()).toString()))
	);

	/**
	 * A plan card's list stamp (`?entry=<plan entry>`): the recipe view, for
	 * this visit only (`borrowListView`), scrolled to that recipe's block, its heading focused
	 * and the block lit once; the address drops the parameter, so Back or a
	 * reload does not jump again.
	 */
	let giveBack: (() => void) | undefined;
	// Leaving the list (to another list too: the page stays mounted then),
	// the view the reader chose comes back.
	const giveViewBack = () => {
		giveBack?.();
		giveBack = undefined;
	};
	onDestroy(giveViewBack);
	beforeNavigate(({ to }) => {
		if (to?.url.pathname !== page.url.pathname) giveViewBack();
	});
	afterNavigate(() => {
		const planEntry = page.url.searchParams.get('entry');
		if (!planEntry) return;
		const url = new URL(page.url.href);
		url.searchParams.delete('entry');
		replaceState(url, page.state);
		const entry = list.entries.find((e) => e.planEntryId === planEntry);
		if (!entry) return;
		giveViewBack();
		giveBack = borrowListView('recipe');
		void tick().then(() => {
			const block = document.querySelector<HTMLElement>(
				`[data-entry-block="${CSS.escape(entry.id)}"]`
			);
			if (!block) return;
			// Its head at the top, clear of the bar (`scroll-mt-*`): a recipe's
			// block is often taller than the screen, and centered, its title
			// scrolled away.
			block.scrollIntoView({ block: 'start', behavior: 'smooth' });
			block.querySelector<HTMLElement>('h2')?.focus({ preventScroll: true });
			// Lit once the page stands still, so its brightest moment is seen:
			// 150ms after the last scroll, or 400ms after the tap when nothing
			// scrolls. Not `scrollend`: the navigation's own scroll ends one
			// before this one starts, and older Safari has none.
			const light = () => {
				removeEventListener('scroll', moved);
				block.dataset.lit = '';
				setTimeout(() => delete block.dataset.lit, 1800);
			};
			let still = setTimeout(light, 400);
			const moved = () => {
				clearTimeout(still);
				still = setTimeout(light, 150);
			};
			addEventListener('scroll', moved, { passive: true });
		});
	});

	let sheetOpen = $state(false);
	let sheetRow = $state<Row | null>(null);
	const sheetDepartment = $derived<Department>(
		list.items.find((i) => i.id === sheetRow?.itemIds[0])?.department ?? 'other'
	);

	$effect(() => {
		shell.breadcrumb = list.name;
		shell.parent = { href: resolve('/lists'), label: m.lists_title() };
		return () => {
			shell.breadcrumb = undefined;
			shell.parent = undefined;
		};
	});

	const owner = $derived(list.owner.id === session.user?.id);
	const people = $derived([list.owner, ...list.members]);
	let shareOpen = $state(false);
	let peopleOpen = $state(false);

	function openSheet(row: Row) {
		sheetRow = row;
		sheetOpen = true;
	}

	async function pickDepartment(department: Department) {
		const row = sheetRow;
		if (!row) return;
		const before = lst.list;
		lst.list = {
			...before,
			items: before.items.map((i) => (row.itemIds.includes(i.id) ? { ...i, department } : i))
		};
		// The override reaches every item of that name, on this list too: the reload brings it.
		await lst.apply(
			setDepartment(row.name, department).then(() => getShoppingList(list.id)),
			() => before
		);
	}

	async function removeSheetRow() {
		if (!sheetRow) return;
		sheetOpen = false;
		await lst.apply(removeItems(list.id, sheetRow.itemIds));
	}

	async function deleteList() {
		try {
			await deleteShoppingList(list.id);
			await goto(resolve('/lists'));
		} catch (error) {
			failList(error);
		}
	}

	async function leave() {
		try {
			await leaveList(list.id);
			toast.success(m.list_left());
			await goto(resolve('/lists'));
		} catch (error) {
			failList(error);
		}
	}

	const changeServings = (entry: ShoppingEntry, servings: number) =>
		lst.apply(setEntryServings(list.id, entry.id, servings));
	const dropEntry = (entry: ShoppingEntry) => lst.apply(removeEntry(list.id, entry.id));

	let importOpen = $state(false);

	/**
	 * Pasted text added items: the answer goes through `apply` like any change,
	 * and "Undo" removes exactly the items it added, through `apply` too, so
	 * a later change's answer still wins.
	 */
	async function imported(answer: ShoppingList, addedIds: string[]) {
		if (!(await lst.apply(Promise.resolve(answer))) || addedIds.length === 0) return;
		const count = addedIds.length;
		toast.success(count === 1 ? m.list_import_added_one() : m.list_import_added({ count }), {
			action: {
				label: m.common_undo(),
				onClick: () => void lst.apply(removeItems(answer.id, addedIds))
			}
		});
	}

	/** The item just added: its row lights up where the list sorted it. */
	let flashId = $state<string | null>(null);
	let flashTimer: ReturnType<typeof setTimeout> | undefined;

	async function add(text: string, fields: { quantity?: number; unit?: string }) {
		const before = new Set(list.items.map((i) => i.id));
		if (!(await lst.apply(addFreeItem(list.id, text, fields)))) return false;
		// The newest of one's own: a member's item may have come in with the same answer.
		const item = list.items.findLast(
			(i) => !before.has(i.id) && i.addedBy?.id === session.user?.id
		);
		if (item) {
			clearTimeout(flashTimer);
			flashId = item.id;
			flashTimer = setTimeout(() => (flashId = null), 1800);
			toast.success(
				m.list_added_to({ name: item.name, department: departmentLabel(item.department) })
			);
		}
		return true;
	}
</script>

<svelte:head><title>{list.name} · {m.app_name()}</title></svelte:head>

<div class="pt-6 pb-8 md:pt-4">
	<!-- Phone: back and ⋯ above the title; desktop: ⋯ beside it. One menu either way. -->
	<header class="mb-4 grid grid-cols-[minmax(0,1fr)_auto] items-end gap-x-4">
		<IconButton
			href={resolve('/lists')}
			label={m.detail_back()}
			class="justify-self-start max-md:size-11 lg:hidden"
		>
			<ArrowLeft class="size-5" aria-hidden="true" />
		</IconButton>
		<div class="col-span-2 row-start-2 mt-5 min-w-0 lg:col-span-1 lg:row-start-1 lg:mt-0">
			<h1 class="font-display text-display-md font-medium wrap-break-word md:text-display-lg">
				{list.name}
			</h1>
			{#if fromPlans.length > 0}
				<!-- Where the planned recipes came from: two lines of stamps, then "+n". -->
				<div class="mt-1.5 flex items-start gap-x-1.5 max-sm:flex-col max-sm:gap-y-0.5">
					<span class="shrink-0 text-caption leading-6 text-text-muted" aria-hidden="true"
						>{m.link_from_plans()}</span
					>
					<StampRow
						label={m.link_from_plans()}
						class="w-full flex-1"
						stamps={fromPlans.map((p) => ({
							key: p.key,
							kind: 'plan' as const,
							label: p.label,
							href: p.href,
							quiet: !p.member
						}))}
					/>
				</div>
			{/if}
			<!--
				Phone: "Shop" is the page's main action, a wide button that also
				carries the counts and fills along its foot as items are ticked.
				From a tablet on, "Shop" leads the header's buttons and the counts
				head the list card, beside the view switch.
			-->
			<a
				href={shopHref(list.id)}
				aria-describedby="list-shop-counts"
				class="relative mt-4 flex h-14 items-center gap-3 overflow-hidden rounded-2xl bg-primary hover-tint px-5 text-primary-foreground shadow-cta transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.99] md:hidden {arrive}"
			>
				<ShoppingBasket class="size-5 shrink-0" aria-hidden="true" />
				<span class="text-body font-semibold">{m.list_shop()}</span>
				<span aria-hidden="true" class="ml-auto text-body-sm whitespace-nowrap opacity-85">
					<span class="max-[359px]:hidden">{m.list_counts({ open: openRows, done: doneRows })}</span
					>
					<span class="min-[360px]:hidden">{m.list_open({ open: openRows })}</span>
				</span>
				<span aria-hidden="true" class="absolute inset-x-0 bottom-0 h-1 bg-primary-foreground/25">
					<span
						class="block h-full bg-primary-foreground transition-[width] duration-300"
						style:width="{(doneRows / Math.max(1, openRows + doneRows)) * 100}%"
					></span>
				</span>
			</a>
			<!-- The counts describe the link without joining its name, "Shop". -->
			<span id="list-shop-counts" class="sr-only"
				>{m.list_counts({ open: openRows, done: doneRows })}</span
			>
		</div>
		<div class="col-start-2 row-start-1 flex items-center gap-2 lg:mb-1 {arrive}">
			<Button href={shopHref(list.id)} class="shrink-0 max-md:hidden">
				<ShoppingBasket class="size-4" aria-hidden="true" />
				{m.list_shop()}
			</Button>
			{#if people.length > 1}
				<!-- Opens who is on the list; from lg on the side card shows them too. -->
				<button
					type="button"
					aria-haspopup="dialog"
					aria-label={m.list_people_show({ count: people.length })}
					onclick={() => (peopleOpen = true)}
					class="relative rounded-pill focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary max-md:after:absolute max-md:after:inset-x-0 max-md:after:-inset-y-2"
				>
					<PeopleStack {people} />
				</button>
			{/if}
			{#if owner}
				<ShareButton
					label={m.list_share()}
					onclick={() => (shareOpen = true)}
					class="max-md:size-11 md:hidden"
				/>
				<Button variant="secondary" onclick={() => (shareOpen = true)} class="max-md:hidden">
					<Share2 class="size-4" aria-hidden="true" />
					{m.list_share()}
				</Button>
			{/if}
			<ListMenu
				name={list.name}
				{owner}
				onshare={() => (shareOpen = true)}
				onaddtext={() => (importOpen = true)}
				onleave={leave}
				onrename={(name) => lst.apply(renameShoppingList(list.id, name))}
				oncopy={() => copyText(listText(list), m.list_copied())}
				ondelete={deleteList}
			/>
		</div>
	</header>

	<div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px] lg:items-start">
		<div class="min-w-0 {arrive}" style:--fade-from={created ? 0.4 : undefined}>
			<AddItemCard onadd={add} focus={created} />
			<ListPanel open={openRows} done={doneRows} {empty}>
				<ListGroups
					{departments}
					{recipes}
					checkers={people.length > 1}
					{flashId}
					stamps
					onToggle={(row) => lst.toggle(row)}
					onOpen={openSheet}
				>
					{#snippet entryMenu(entry)}
						<EntryMenu
							{entry}
							onservings={(n) => changeServings(entry, n)}
							onremove={() => dropEntry(entry)}
						/>
					{/snippet}
				</ListGroups>

				{#if done.length > 0}
					<section class="mt-5">
						<div class="flex flex-wrap items-center justify-between gap-x-2">
							<h2 class="font-display text-body-lg font-medium text-text-muted italic">
								{m.list_done_heading({ count: done.length })}
							</h2>
							<!-- A 44px target grown upward: the first done row's target starts right below.
								     Its padding is given back on the right, so its text ends where the
								     counts above end; with 200% text it wraps to its own line, at the end. -->
							<button
								type="button"
								onclick={() => lst.apply(clearChecked(list.id))}
								class="relative -mr-1 ml-auto h-8 rounded-pill px-1 text-body-sm font-semibold whitespace-nowrap text-primary transition after:absolute after:inset-x-0 after:-top-[13px] after:bottom-0 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
							>
								{m.list_clear_checked()}
							</button>
						</div>
						<ul>
							{#each done as row (row.key)}
								<ShoppingRow
									checkers={people.length > 1}
									{row}
									flash={!!flashId && row.itemIds.includes(flashId)}
									onToggle={(row) => lst.toggle(row)}
									onOpen={openSheet}
								/>
							{/each}
						</ul>
					</section>
				{/if}
			</ListPanel>
		</div>

		<aside class="min-w-0 max-lg:hidden {arrive}">
			<ListSide
				{list}
				onservings={changeServings}
				onremove={dropEntry}
				onshare={owner ? () => (shareOpen = true) : undefined}
			/>
		</aside>
	</div>
</div>

<PeopleSheet
	bind:open={peopleOpen}
	owner={list.owner}
	members={list.members}
	onshare={owner ? () => (shareOpen = true) : undefined}
/>

<ShareDialog
	bind:open={shareOpen}
	title={m.list_share_title({ name: list.name })}
	text={m.list_share_text()}
	owner={list.owner}
	members={list.members}
	pending={list.pending}
	invite={(userId) => inviteToList(list.id, userId)}
	remove={(userId) => removeListMember(list.id, userId)}
	apply={(request, onfail) => lst.apply(request, undefined, onfail)}
	ongone={failList}
/>

<ListImportDialog bind:open={importOpen} mode={{ kind: 'add', list }} ondone={imported} />

<ItemSheet
	bind:open={sheetOpen}
	row={sheetRow}
	department={sheetDepartment}
	onpick={pickDepartment}
	onremove={removeSheetRow}
/>
