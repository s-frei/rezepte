<script lang="ts">
	import CalendarDays from '@lucide/svelte/icons/calendar-days';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import House from '@lucide/svelte/icons/house';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import { parseDate, type DateValue } from '@internationalized/date';
	import { Dialog, RangeCalendar, type DateRange } from 'bits-ui';
	import { tick, untrack } from 'svelte';
	import { on } from 'svelte/events';
	import { SvelteMap, SvelteSet } from 'svelte/reactivity';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import {
		getEntryShoppingPreview,
		getShoppingPreview,
		putPlanOnList,
		setStaple,
		type PlanEntry,
		type PreviewEntry
	} from '#lib/api/mealplan.js';
	import { listShoppingLists, type ShoppingSummary } from '#lib/api/shopping.js';
	import Button from '#lib/components/ui/Button.svelte';
	import Checkbox from '#lib/components/ui/Checkbox.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import ResponsiveDialog from '#lib/components/ui/ResponsiveDialog.svelte';
	import Select from '#lib/components/ui/Select.svelte';
	import Switch from '#lib/components/ui/Switch.svelte';
	import type { PlanState } from '#lib/mealplan/plan-state.svelte.js';
	import { dayLabel, dayNames, spanLabel } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { formatQuantity } from '#lib/recipe/format.js';
	import { pickKey } from '#lib/shopping/add.js';
	import { listHref } from '#lib/shopping/href.js';
	import { byListName, quotedList } from '#lib/shopping/plan-ref.js';
	import { load, save } from '#lib/storage.js';

	/**
	 * "Put on the list": the days (today to the shown week's Sunday, or a
	 * past or later week whole), optionally the tray, the list, and per
	 * recipe its ingredients - all picked but the plan's staples. Recipes on
	 * a list the reader can open already are left out and counted in one
	 * line; the server skips them too. A changed range or tray refetches the
	 * preview, and a recipe kept in it keeps the reader's picks.
	 *
	 * `only`: one entry, from its ⋯. No days; its ingredients open and its
	 * recipe always picked; the lists it is on already are left out of the
	 * choice, and the server puts it on another list too (`alsoListed`).
	 */
	let {
		open = $bindable(false),
		week,
		today,
		only = null
	}: {
		open?: boolean;
		week: PlanState;
		today: string;
		only?: PlanEntry | null;
	} = $props();

	const LAST_PLAN_LIST = 'rezepte:last-plan-list';
	const LAST_LIST = 'rezepte:last-list';
	const uid = $props.id();
	const locale = getLocale();
	const plan = $derived(week.plan);

	/** The days asked for, both ends included; the calendar's own value runs ahead while half picked. */
	let span = $state({ from: '', to: '' });
	let calendar = $state<DateRange>({ start: undefined, end: undefined });
	let picking = $state(false);
	let tray = $state(false);

	let lists = $state<ShoppingSummary[]>([]);
	let listsLoaded = $state(false);
	let listsFailed = $state(false);
	let listId = $state('');
	let creating = $state(false);
	let newName = $state('');
	/** The reader typed a name; until then it follows the days ("Einkauf 23.–29. Nov."). */
	let named = $state(false);

	let entries = $state<PreviewEntry[]>([]);
	let previewLoaded = $state(false);
	/** Why the preview is missing: the request failed, or the days span too far (a 422). */
	let previewFailed = $state<'' | 'error' | 'range'>('');
	/** A changed range, tray or staple is being fetched: the recipes shown are the old ones. */
	let refreshing = $state(false);
	/** Picked ingredients per entry; an entry with none stays off the list. */
	const picks = new SvelteMap<string, SvelteSet<string>>();
	const expanded = new SvelteSet<string>();
	let saving = $state(false);

	/** Tray recipes the switch would add: on no list the reader can open yet. */
	const trayRecipes = $derived(
		plan.entries.filter((e) => e.day === null && e.recipeId && e.lists.length === 0).length
	);
	const fresh = $derived(only ? entries : entries.filter((e) => !e.alreadyListed));
	const listed = $derived(only ? [] : entries.filter((e) => e.alreadyListed));
	const chosen = $derived(fresh.filter((e) => (picks.get(e.entryId)?.size ?? 0) > 0));
	/** The lists to pick from: for one entry, those it is not on yet. */
	const choices = $derived(
		only ? byListName(lists.filter((l) => !only.lists.some((on) => on.id === l.id))) : lists
	);
	const showNew = $derived(creating || (listsLoaded && !listsFailed && choices.length === 0));
	const canSubmit = $derived(
		chosen.length > 0 &&
			!listsFailed &&
			previewLoaded &&
			!previewFailed &&
			!refreshing &&
			(showNew ? !!newName.trim() : !!listId) &&
			!saving
	);

	/** The list the one-entry button names: "Auf „Vorrat“ setzen". */
	const target = $derived(
		showNew ? newName.trim() : (lists.find((l) => l.id === listId)?.name ?? '')
	);

	/** "2 recipes are already on “Weekly shop” and “Market”": every list the shown week says they are on. */
	const alreadyLine = $derived.by(() => {
		if (only)
			return only.lists.length > 0
				? m.plan_to_list_one_already({
						names: quotedList(byListName(only.lists).map((l) => l.name))
					})
				: '';
		const count = listed.length;
		if (count === 0) return '';
		// Every list they are on, not only the first of each.
		const names = new Set(
			listed.flatMap(
				(e) => plan.entries.find((p) => p.id === e.entryId)?.lists.map((l) => l.name) ?? []
			)
		);
		if (names.size > 0) {
			const all = quotedList([...names].sort((a, b) => a.localeCompare(b, locale)));
			return count === 1
				? m.plan_to_list_already_named_one({ names: all })
				: m.plan_to_list_already_named({ count, names: all });
		}
		return count === 1 ? m.plan_to_list_already_one() : m.plan_to_list_already({ count });
	});

	const daysText = $derived(
		!span.from
			? ''
			: span.from === span.to
				? dayLabel(span.from, locale)
				: `${dayLabel(span.from, locale)} – ${dayLabel(span.to, locale)}`
	);
	/** "Sa" in the shown week, "Mo 30. Nov." beyond it, "Irgendwann" on the tray. */
	function dayOf(day: string | null): string {
		if (day === null) return m.plan_tray();
		const names = dayNames(day, locale);
		return day >= plan.from && day <= plan.to ? names.weekday : `${names.weekday} ${names.date}`;
	}

	// A fresh start on every opening.
	$effect(() => {
		if (!open) return;
		untrack(start);
	});

	function start() {
		const sunday = plan.to;
		const from = today >= plan.from && today <= sunday ? today : plan.from;
		span = { from, to: sunday };
		calendar = { start: parseDate(from), end: parseDate(sunday) };
		picking = false;
		// The tray's recipes on no list count in the button's badge: they are in from the start.
		tray = !only && trayRecipes > 0;
		creating = false;
		named = false;
		newName = m.plan_new_list_name({ span: spanLabel(from, sunday, locale) });
		picks.clear();
		expanded.clear();
		if (only) expanded.add(only.id);
		entries = [];
		void loadLists();
		void loadPreview();
	}

	let listsReq = 0;
	async function loadLists() {
		const mine = ++listsReq;
		listsLoaded = false;
		listsFailed = false;
		try {
			const all = (await listShoppingLists()).items;
			if (mine !== listsReq) return;
			lists = all;
			// The list the plan went to last, else the one the week went to,
			// else the last one used at all; none of them: a new one.
			const pool = only ? all.filter((l) => !only.lists.some((on) => on.id === l.id)) : all;
			const known = [load(LAST_PLAN_LIST), plan.lists[0]?.id, load(LAST_LIST)].find((id) =>
				pool.some((l) => l.id === id)
			);
			listId = known ?? pool[0]?.id ?? '';
			// One entry: any list it is not on will do; a new one only when there is none.
			creating = only ? pool.length === 0 : !known;
		} catch (error) {
			if (mine !== listsReq) return;
			listsFailed = true;
			if (!isSignedOut(error)) toast.error(m.lists_load_error());
		} finally {
			if (mine === listsReq) listsLoaded = true;
		}
	}

	/** What the server would put on a list; `keep` merges into the picks made so far. */
	let previewReq = 0;
	async function loadPreview(keep = false) {
		const mine = ++previewReq;
		if (!keep) previewLoaded = false;
		refreshing = keep;
		previewFailed = '';
		const before = new Map(entries.map((e) => [e.entryId, e]));
		try {
			const answer = (
				await (only
					? getEntryShoppingPreview(plan.id, only.id)
					: getShoppingPreview(plan.id, span, tray))
			).entries;
			if (mine !== previewReq) return;
			for (const entry of answer) {
				const old = before.get(entry.entryId);
				const set = picks.get(entry.entryId);
				if (!old || !set) {
					// New in the range: everything but the staples.
					picks.set(
						entry.entryId,
						new SvelteSet(
							entry.ingredients.filter((i) => !i.staple).map((i) => pickKey(i.group, i.index))
						)
					);
					continue;
				}
				// Kept: the reader's picks stay, but a staple marked or cleared since follows.
				for (const ing of entry.ingredients) {
					const was = old.ingredients.find((o) => o.group === ing.group && o.index === ing.index);
					if (was && was.staple !== ing.staple) {
						const key = pickKey(ing.group, ing.index);
						if (ing.staple) set.delete(key);
						else set.add(key);
					}
				}
			}
			entries = answer;
			previewLoaded = true;
		} catch (error) {
			if (mine !== previewReq) return;
			// Said in place, with its retry: recipes of the days before must not stay.
			// A range the server refuses gets no retry; other days are the way out.
			previewFailed = error instanceof ApiError && error.status === 422 ? 'range' : 'error';
		} finally {
			if (mine === previewReq) refreshing = false;
		}
	}

	/** Escape in the open calendar folds it, before the dialog's own Escape (a document listener) sees it. */
	function foldOnEscape(node: HTMLElement) {
		return on(node, 'keydown', (event) => {
			if (event.key !== 'Escape') return;
			event.stopPropagation();
			picking = false;
			void tick().then(() => document.getElementById(`${uid}-days`)?.focus());
		});
	}

	/** The server takes ranges up to 62 days apart; the calendar offers 62 days, both ends counted. */
	const MAX_DAYS = 62;
	/** With one end picked, the days that would make the range too long. */
	const tooFar = (date: DateValue) =>
		!!calendar.start && !calendar.end && Math.abs(date.compare(calendar.start)) >= MAX_DAYS;

	function pickRange(next: DateRange) {
		calendar = next;
		if (!next.start || !next.end) return;
		span = { from: next.start.toString(), to: next.end.toString() };
		if (!named) newName = m.plan_new_list_name({ span: spanLabel(span.from, span.to, locale) });
		picking = false;
		void loadPreview(true);
		// The calendar folds away; the keyboard lands on what opened it.
		void tick().then(() => document.getElementById(`${uid}-days`)?.focus());
	}

	/** On: everything but the staples (all, when the recipe holds nothing else). */
	function toggleRecipe(entry: PreviewEntry, on: boolean) {
		const wanted = entry.ingredients.filter((i) => !i.staple);
		const keys = (wanted.length ? wanted : entry.ingredients).map((i) => pickKey(i.group, i.index));
		picks.set(entry.entryId, new SvelteSet(on ? keys : []));
	}

	function toggleIngredient(entryId: string, key: string, on: boolean) {
		const set = picks.get(entryId);
		if (!set) return;
		if (on) set.add(key);
		else set.delete(key);
	}

	let stapling = $state('');
	async function toggleStaple(entry: PreviewEntry, ing: PreviewEntry['ingredients'][number]) {
		const key = pickKey(ing.group, ing.index);
		const next = !ing.staple;
		stapling = `${entry.entryId}/${key}`;
		try {
			await setStaple(plan.id, ing.name, next);
			// The row as shown now - a range refetch may have replaced it meanwhile
			// - at once; the same ingredient in the other recipes with the refetch,
			// which also supersedes one still in flight.
			const row = entries
				.find((e) => e.entryId === entry.entryId)
				?.ingredients.find((i) => i.group === ing.group && i.index === ing.index);
			if (row) {
				row.staple = next;
				toggleIngredient(entry.entryId, key, !next);
			}
			await loadPreview(true);
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.plan_entry_error());
		} finally {
			stapling = '';
		}
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!canSubmit) return;
		saving = true;
		const name = showNew ? newName.trim() : (lists.find((l) => l.id === listId)?.name ?? '');
		const count = chosen.length;
		const request = putPlanOnList(
			plan.id,
			{
				...(showNew ? { newListName: name } : { listId }),
				...(only ? { alsoListed: true } : {}),
				picks: chosen.map((e) => ({
					entryId: e.entryId,
					ingredients: e.ingredients
						.filter((i) => picks.get(e.entryId)?.has(pickKey(i.group, i.index)))
						.map(({ group, index }) => ({ group, index }))
				}))
			},
			week.range
		);
		const done = await week.apply(
			request.then((answer) => answer.plan),
			undefined,
			(error) => {
				if (!isSignedOut(error)) toast.error(m.plan_to_list_error());
			}
		);
		saving = false;
		if (!done) return;
		const { listId: target } = await request;
		save(LAST_PLAN_LIST, target);
		save(LAST_LIST, target);
		open = false;
		toast.success(
			count === 1
				? m.plan_to_list_added_one({ list: name })
				: m.plan_to_list_added({ count, list: name }),
			{ action: { label: m.add_to_list_open(), onClick: () => goto(listHref(target)) } }
		);
	}

	const label = 'text-label font-bold tracking-[0.08em] text-text-muted uppercase';
	const link =
		'-mx-2 -mt-4 -mb-2 rounded-md px-2 pt-4 pb-2 text-caption font-semibold text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
	const field =
		'flex min-h-11 w-full items-center gap-2.5 rounded-md border border-border bg-surface-elevated px-4 py-1.5 text-left text-body-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
	const navButton =
		'flex size-11 items-center justify-center rounded-full text-text-muted hover-tint focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:size-9';
</script>

{#snippet retry(text: string, again: () => void)}
	<button type="button" onclick={again} class="{field} justify-between text-text-muted">
		<span class="min-w-0 text-caption leading-tight">{text}</span>
		<span class="shrink-0 pl-3 font-semibold whitespace-nowrap text-primary"
			>{m.add_to_list_retry()}</span
		>
	</button>
{/snippet}

{#snippet body()}
	<Dialog.Title class="font-display text-heading font-medium"
		>{!only
			? m.plan_to_list()
			: only.lists.length > 0
				? m.plan_to_list_one_title_another()
				: m.plan_to_list_one_title()}</Dialog.Title
	>
	{#if only}
		<!-- The recipe below names the meal; said once for assistive technology. -->
		<Dialog.Description class="sr-only">{only.title} · {dayOf(only.day)}</Dialog.Description>
	{/if}
	<div class="mt-5 space-y-5">
		{#if !only}
			<div>
				<span id="{uid}-days-label" class="mb-2 block {label}">{m.plan_to_list_days()}</span>
				<button
					id="{uid}-days"
					type="button"
					aria-labelledby="{uid}-days-label {uid}-days-value"
					aria-expanded={picking}
					aria-controls="{uid}-calendar"
					onclick={() => (picking = !picking)}
					class={field}
				>
					<CalendarDays class="size-4 shrink-0 text-text-muted" aria-hidden="true" />
					<span id="{uid}-days-value" class="min-w-0 flex-1 font-medium">{daysText}</span>
					<ChevronDown
						class="size-4 shrink-0 text-text-muted transition-transform {picking
							? 'rotate-180'
							: ''}"
						aria-hidden="true"
					/>
				</button>
				{#if picking}
					<!-- Each day is a seventh of the width: at 320px that is 40px, a
				     row of adjoining targets with no gap to miss into. -->
					<div {@attach foldOnEscape}>
						<RangeCalendar.Root
							id="{uid}-calendar"
							value={calendar}
							onValueChange={pickRange}
							isDateDisabled={tooFar}
							weekStartsOn={1}
							{locale}
							fixedWeeks
							weekdayFormat="short"
							class="mt-2 rounded-2xl border border-border bg-surface-elevated p-2"
						>
							{#snippet children({ months, weekdays })}
								<RangeCalendar.Header class="flex items-center justify-between pb-1">
									<RangeCalendar.PrevButton class={navButton}>
										<ChevronLeft class="size-4.5" aria-hidden="true" />
									</RangeCalendar.PrevButton>
									<RangeCalendar.Heading class="font-display text-body font-medium" />
									<RangeCalendar.NextButton class={navButton}>
										<ChevronRight class="size-4.5" aria-hidden="true" />
									</RangeCalendar.NextButton>
								</RangeCalendar.Header>
								{#each months as month (month.value.toString())}
									<RangeCalendar.Grid class="w-full border-collapse">
										<RangeCalendar.GridHead>
											<RangeCalendar.GridRow class="flex">
												{#each weekdays as weekday, i (i)}
													<RangeCalendar.HeadCell
														class="flex-1 pb-1 text-center text-micro font-semibold text-text-muted"
													>
														{weekday.replace(/\.$/, '')}
													</RangeCalendar.HeadCell>
												{/each}
											</RangeCalendar.GridRow>
										</RangeCalendar.GridHead>
										<RangeCalendar.GridBody>
											{#each month.weeks as days, w (w)}
												<RangeCalendar.GridRow class="flex">
													{#each days as date (date.toString())}
														<RangeCalendar.Cell {date} month={month.value} class="flex-1 p-0">
															<RangeCalendar.Day
																class="flex h-11 w-full items-center justify-center text-body-sm text-text tabular-nums transition-colors focus-visible:relative focus-visible:z-10 focus-visible:outline-2 focus-visible:outline-primary data-[disabled]:cursor-not-allowed data-[disabled]:text-text-muted/40 data-[outside-month]:text-text-muted/60 data-[range-middle]:bg-accent data-[range-middle]:text-accent-foreground data-[selected]:bg-accent data-[selection-end]:rounded-r-md data-[selection-end]:bg-primary data-[selection-end]:font-semibold data-[selection-end]:text-primary-foreground data-[selection-start]:rounded-l-md data-[selection-start]:bg-primary data-[selection-start]:font-semibold data-[selection-start]:text-primary-foreground data-[today]:font-bold data-[today]:underline data-[today]:decoration-2 data-[today]:underline-offset-4 md:h-10 [&:not([data-selected])]:rounded-md [&:not([data-selected])]:hover:bg-background"
															/>
														</RangeCalendar.Cell>
													{/each}
												</RangeCalendar.GridRow>
											{/each}
										</RangeCalendar.GridBody>
									</RangeCalendar.Grid>
								{/each}
							{/snippet}
						</RangeCalendar.Root>
					</div>
				{/if}
				{#if trayRecipes > 0}
					<div class="mt-3 flex items-center justify-between gap-3">
						<span class="min-w-0">
							<span class="block text-body-sm font-medium">{m.plan_to_list_tray()}</span>
							<span class="block text-caption text-text-muted"
								>{trayRecipes === 1
									? m.plan_to_list_tray_count_one()
									: m.plan_to_list_tray_count({ count: trayRecipes })}</span
							>
						</span>
						<Switch
							checked={tray}
							label={m.plan_to_list_tray()}
							onchange={(on) => {
								tray = on;
								void loadPreview(true);
							}}
						/>
					</div>
				{/if}
			</div>
		{/if}

		<div>
			<div class="mb-2 flex items-baseline justify-between">
				<span class={label}>{m.add_to_list_list_label()}</span>
				{#if choices.length > 0 && !listsFailed}
					<button type="button" onclick={() => (creating = !creating)} class={link}>
						{creating ? m.common_cancel() : m.add_to_list_new_list()}
					</button>
				{/if}
			</div>
			{#if listsFailed}
				{@render retry(m.lists_load_error(), loadLists)}
			{:else if showNew}
				<Input
					id="{uid}-name"
					label={m.lists_new_name_label()}
					placeholder={m.lists_new_name_placeholder()}
					autocomplete="off"
					maxlength={80}
					bind:value={newName}
					oninput={() => (named = true)}
				/>
				{#if only && choices.length === 0}
					<p class="mt-2 text-caption text-text-muted">{m.plan_to_list_one_all()}</p>
				{/if}
			{:else}
				<Select
					bind:value={listId}
					options={choices.map((l) => ({ value: l.id, label: l.name }))}
					label={m.add_to_list_list_label()}
					class="w-full justify-between bg-surface-elevated"
				/>
			{/if}
			{#if only && alreadyLine}
				<p class="mt-2 flex items-center gap-2 text-caption text-success-foreground">
					<ShoppingBasket class="size-4 shrink-0" aria-hidden="true" />
					{alreadyLine}
				</p>
			{/if}
		</div>

		<div>
			{#if fresh.length > 0 && !previewFailed && !only}
				<span class="mb-2 block {label}"
					>{m.plan_to_list_recipes({ selected: chosen.length, total: fresh.length })}</span
				>
			{/if}
			{#if previewFailed === 'range'}
				<p
					class="flex min-h-11 items-center rounded-md border border-border bg-surface-elevated px-4 py-1.5 text-caption leading-tight text-text-muted"
				>
					{m.plan_to_list_range_error()}
				</p>
			{:else if previewFailed}
				{@render retry(m.plan_to_list_load_error(), () => loadPreview())}
			{:else if !previewLoaded}
				<div class="h-28 animate-pulse rounded-2xl bg-skeleton" aria-hidden="true"></div>
			{:else}
				{#if fresh.length > 0}
					<!-- The frame rounds and clips; on a computer the recipes scroll inside
					     it, so the dialog's buttons stay in view (as AddToListDialog). -->
					<div
						aria-busy={refreshing}
						class="overflow-hidden rounded-2xl border border-border bg-surface-elevated transition-opacity {refreshing
							? 'opacity-55'
							: ''}"
					>
						<ul class="md:max-h-[40dvh] md:overflow-y-auto md:overscroll-contain">
							{#each fresh as entry (entry.entryId)}
								{@const set = picks.get(entry.entryId)}
								{@const count = set?.size ?? 0}
								{@const day = dayOf(entry.day)}
								{@const isOpen = expanded.has(entry.entryId)}
								<li class="border-b border-border last:border-b-0">
									<div class="flex items-center gap-1 pr-1.5 pl-3.5">
										<label
											class="flex min-h-14 min-w-0 flex-1 cursor-pointer items-center gap-3 py-2"
										>
											<!-- One entry: its recipe stays picked, only its ingredients can go. -->
											{#if !only}
												<Checkbox
													bind:checked={() => count > 0, (on) => toggleRecipe(entry, on)}
													label={entry.title}
												/>
											{/if}
											<span class="min-w-0 {count > 0 ? 'text-text' : 'text-text-muted'}">
												<span class="block font-semibold break-words [hyphens:none]"
													>{entry.title}</span
												>
												<span class="block text-caption text-text-muted"
													>{day} · {entry.servings === 1
														? m.plan_servings_one()
														: m.plan_servings({ count: entry.servings })}</span
												>
											</span>
										</label>
										<button
											type="button"
											aria-label={m.plan_to_list_ingredients({ title: entry.title })}
											aria-expanded={isOpen}
											aria-controls="{uid}-ing-{entry.entryId}"
											onclick={() =>
												isOpen ? expanded.delete(entry.entryId) : expanded.add(entry.entryId)}
											class="flex min-h-11 shrink-0 items-center gap-1 rounded-md hover-tint px-2 text-caption whitespace-nowrap text-text-muted tabular-nums focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
										>
											{m.plan_to_list_picked({ selected: count, total: entry.ingredients.length })}
											<ChevronDown
												class="size-4 transition-transform {isOpen ? 'rotate-180' : ''}"
												aria-hidden="true"
											/>
										</button>
									</div>
									{#if isOpen}
										<ul
											id="{uid}-ing-{entry.entryId}"
											class="mx-3.5 mb-2 border-t border-dashed border-border"
										>
											{#each entry.ingredients as ing (pickKey(ing.group, ing.index))}
												{@const key = pickKey(ing.group, ing.index)}
												{@const on = set?.has(key) ?? false}
												<li
													class="flex items-start gap-1 border-b border-dashed border-border last:border-b-0"
												>
													<label
														class="flex min-h-11 min-w-0 flex-1 cursor-pointer items-start gap-3 py-2.5 text-body-sm {on
															? 'text-text'
															: 'text-text-muted'}"
													>
														<span class="mt-[calc((1lh_-_20px)/2)] flex">
															<Checkbox
																bind:checked={
																	() => on, (v) => toggleIngredient(entry.entryId, key, v)
																}
																label={ing.name}
															/>
														</span>
														<span class="min-w-0 break-words">
															{ing.quantity === undefined ? '' : formatQuantity(ing.quantity)}
															{ing.unit ?? ''}
															{ing.name}
															{#if ing.staple}
																<span class="block text-caption text-text-muted"
																	>{m.plan_staple()}</span
																>
															{/if}
														</span>
													</label>
													<button
														type="button"
														aria-label={m.plan_staple_toggle({ name: ing.name })}
														aria-pressed={ing.staple}
														title={m.plan_staple()}
														disabled={stapling !== ''}
														onclick={() => toggleStaple(entry, ing)}
														class="flex size-11 shrink-0 items-center justify-center rounded-full hover-tint transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-60 md:size-10 {ing.staple
															? 'text-primary'
															: 'text-text-muted'}"
													>
														<House
															class="size-4"
															fill={ing.staple ? 'currentColor' : 'none'}
															fill-opacity={ing.staple ? 0.18 : 0}
															aria-hidden="true"
														/>
													</button>
												</li>
											{/each}
										</ul>
									{/if}
								</li>
							{/each}
						</ul>
					</div>
					<p class="mt-2 text-caption text-text-muted">{m.plan_to_list_hint()}</p>
				{:else}
					<p
						class="rounded-2xl border border-dashed border-handle px-4 py-5 text-center text-body-sm text-text-muted"
					>
						{listed.length > 0 ? m.plan_to_list_nothing_new() : m.plan_to_list_none()}
					</p>
				{/if}
				{#if alreadyLine && !only}
					<p class="mt-3 flex items-center gap-2 text-caption text-success-foreground">
						<ShoppingBasket class="size-4 shrink-0" aria-hidden="true" />
						{alreadyLine}
					</p>
				{/if}
			{/if}
		</div>
	</div>
{/snippet}

{#snippet footer()}
	<Button variant="secondary" class="px-3 max-[340px]:px-2.5 md:px-5" onclick={() => (open = false)}
		>{m.common_cancel()}</Button
	>
	<Button
		type="submit"
		busy={saving}
		disabled={!canSubmit}
		class="min-w-0 grow px-3 md:grow-0 md:px-5 {only
			? 'max-md:h-auto! max-md:min-h-12 max-md:py-2 max-md:text-center max-md:text-balance md:whitespace-nowrap'
			: 'whitespace-nowrap'}"
	>
		<ShoppingBasket class="size-4 shrink-0 {only ? '' : 'max-[340px]:hidden'}" aria-hidden="true" />
		{only && chosen.length === 1 && !previewFailed && target
			? m.plan_to_list_one_submit({ list: target })
			: chosen.length === 0 || previewFailed
				? m.plan_to_list()
				: chosen.length === 1
					? m.plan_to_list_submit_one()
					: m.plan_to_list_submit({ count: chosen.length })}
	</Button>
{/snippet}

<ResponsiveDialog
	bind:open
	closeLabel={m.common_cancel()}
	onsubmit={submit}
	footerClass="flex gap-2 px-4 pt-3.5 min-[340px]:px-5 {only
		? 'max-md:flex-col-reverse max-md:*:w-full'
		: ''}"
	{body}
	{footer}
/>
