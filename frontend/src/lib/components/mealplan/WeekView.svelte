<script lang="ts">
	import Plus from '@lucide/svelte/icons/plus';
	import { tick } from 'svelte';
	import { TRIGGERS, type DndEvent } from 'svelte-dnd-action';
	import { toast } from 'svelte-sonner';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import {
		movePlanEntry,
		removePlanEntry,
		restorePlanEntry,
		setPlanServings,
		type PlanEntry
	} from '#lib/api/mealplan.js';
	import { byZone, placeEntry, TRAY } from '#lib/mealplan/entries.js';
	import type { PlanState } from '#lib/mealplan/plan-state.svelte.js';
	import {
		dayNames,
		daysFrom,
		mondayOf,
		pastDays,
		shiftDay,
		weekDays
	} from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { applyDndAriaStrings, applyDrag } from '#lib/recipe/dnd.js';
	import { narrow } from '#lib/ui/narrow.js';
	import DayColumn from './DayColumn.svelte';
	import DayRow from './DayRow.svelte';
	import DayStrip from './DayStrip.svelte';
	import EntryZone from './EntryZone.svelte';
	import MoveSheet from './MoveSheet.svelte';
	import PastDays from './PastDays.svelte';
	import TrayStrip from './TrayStrip.svelte';

	/**
	 * The plan's days and its tray. From `md` every loaded day is a column of
	 * one strip (`DayStrip`) that scrolls by the day, `start` its first day in
	 * view; on a phone the week of `monday` is a card per day, and this week's
	 * days gone by fold into one row (`expanded` opens it). Every day and the
	 * tray is a drop zone; moves, servings and removals change the plan at
	 * once and go through `PlanState.apply`, which puts a refused one back.
	 */
	let {
		week,
		today,
		monday,
		start = $bindable(),
		count = 7,
		expanded = $bindable(false),
		onsettle,
		onat,
		onadd
	}: {
		week: PlanState;
		today: string;
		/** A phone's week. */
		monday: string;
		/** The strip's first day in view. */
		start: string;
		onat?: (at: number) => void;
		/** Columns in view from `md`. */
		count?: number;
		expanded?: boolean;
		onsettle?: (first: string) => void;
		/** Opens the add sheet on a day, or the tray (null). */
		onadd: (day: string | null) => void;
	} = $props();

	let strip = $state<DayStrip>();
	export const aim = (day: string) => strip?.aim(day);
	export const scrollByDays = (by: number) => strip?.scrollByDays(by);
	export const seekDay = (day: number) => strip?.seekDay(day);
	export const aimed = () => strip?.aimed();
	export const release = () => strip?.release();

	applyDndAriaStrings();
	const locale = getLocale();
	const plan = $derived(week.plan);
	const days = $derived(narrow.current ? weekDays(monday) : daysFrom(plan.from, plan.to));
	/** The days in view: a phone's week, the strip's window. */
	const shown = $derived(narrow.current ? days : daysFrom(start, shiftDay(start, count - 1)));
	const zones = $derived(byZone(plan.entries, days));
	const past = $derived(narrow.current ? pastDays(monday, today) : []);
	const pastCount = $derived(past.reduce((n, day) => n + zones[day].length, 0));

	/** While a card is dragged, the zones as the drag shows them. */
	let dragged = $state<Record<string, PlanEntry[]> | null>(null);
	const items = (key: string) => dragged?.[key] ?? zones[key];

	function failed(error: unknown) {
		if (isSignedOut(error)) return;
		// The entry is gone under the page (a member removed it): show what is there.
		if (error instanceof ApiError && error.status === 404) void week.refresh();
		else toast.error(m.plan_entry_error());
	}

	function consider(key: string, event: CustomEvent<DndEvent<PlanEntry>>) {
		if (event.detail.info.trigger === TRIGGERS.DRAG_STARTED) navigator.vibrate?.(10);
		dragged ??= { ...zones };
		applyDrag(event, items(key), (next) => {
			if (dragged) dragged[key] = next;
		});
	}

	/**
	 * A drop ends in two zones, where it left and where it landed; the
	 * landing one (it holds the card) saves the move.
	 */
	function finalize(key: string, event: CustomEvent<DndEvent<PlanEntry>>) {
		applyDrag(event, items(key), (next) => {
			if (dragged) dragged[key] = next;
		});
		const { id } = event.detail.info;
		const index = event.detail.items.findIndex((item) => item.id === id);
		if (index === -1) return;
		dragged = null;
		const day = key === TRAY ? null : key;
		const entry = plan.entries.find((e) => e.id === id);
		if (!entry || (entry.day === day && zones[key].indexOf(entry) === index)) return;
		move(entry, day, index);
	}

	function move(entry: PlanEntry, day: string | null, index?: number) {
		const before = week.plan;
		week.plan = {
			...before,
			entries: placeEntry(before.entries, entry.id, day, index ?? Number.MAX_SAFE_INTEGER)
		};
		void week.apply(
			movePlanEntry(before.id, entry.id, day, index, week.range),
			() => before,
			failed
		);
	}

	/**
	 * Where the keyboard goes once a menu's change has redrawn the week: a
	 * moved or removed card takes its ⋯ with it, and the focus would drop to
	 * the page. After the redraw and a frame, so a closing menu or sheet has
	 * let go of it first.
	 */
	function focusAfter(find: () => HTMLElement | null) {
		void tick().then(() => requestAnimationFrame(() => find()?.focus()));
	}
	const menuOf = (id: string) =>
		document.querySelector<HTMLElement>(`[data-entry-id="${CSS.escape(id)}"] [data-entry-menu]`);
	/** A day's "+", or the tray itself. */
	const addOf = (day: string | null) =>
		document.querySelector<HTMLElement>(
			day === null ? '[data-tray]' : `[data-day-add="${CSS.escape(day)}"]`
		);
	/** Set by a pick in the move sheet, taken when the sheet hands back the focus. */
	let afterMove: (() => HTMLElement | null) | null = null;

	let moving = $state<PlanEntry | null>(null);
	let moveOpen = $state(false);

	function moved(entry: PlanEntry, day: string | null) {
		if (entry.day === day) return;
		// Its ⋯ in the new place; moved out of the week, the "+" of the day it left.
		afterMove = () => menuOf(entry.id) ?? addOf(entry.day);
		move(entry, day);
		// Out of view, the card leaves the page: say where it went.
		if (day !== null && !shown.includes(day))
			toast.success(m.plan_moved({ title: entry.title, day: dayNames(day, locale).full }));
		if (day !== null && past.includes(day)) expanded = true;
	}

	function servings(entry: PlanEntry, count: number) {
		const before = week.plan;
		week.plan = {
			...before,
			entries: before.entries.map((e) => (e.id === entry.id ? { ...e, servings: count } : e))
		};
		void week.apply(setPlanServings(before.id, entry.id, count, week.range), () => before, failed);
	}

	async function remove(entry: PlanEntry) {
		const before = week.plan;
		// The next card's ⋯, else the one before, else the day's "+".
		const zone = zones[entry.day ?? TRAY];
		const at = zone.findIndex((e) => e.id === entry.id);
		const neighbor = zone[at + 1] ?? zone[at - 1];
		week.plan = { ...before, entries: before.entries.filter((e) => e.id !== entry.id) };
		focusAfter(() => (neighbor ? menuOf(neighbor.id) : null) ?? addOf(entry.day));
		const done = await week.apply(
			removePlanEntry(before.id, entry.id, week.range),
			() => before,
			failed
		);
		if (!done) return;
		// Lists follow the plan: say which of the reader's lost it too.
		const { title, lists } = entry;
		const said =
			lists.length === 0
				? m.plan_entry_removed({ title })
				: lists.length === 1
					? m.plan_entry_removed_listed({ title, list: lists[0].name })
					: m.plan_entry_removed_lists({ title, count: lists.length });
		toast.success(said, {
			action: {
				label: m.common_undo(),
				// Back in its place, the keyboard goes back to it: its ⋯, else its day's "+".
				onClick: async () => {
					if (
						!(await week.apply(
							restorePlanEntry(before.id, entry.id, week.range),
							undefined,
							failed
						))
					)
						return;
					if (entry.day !== null && past.includes(entry.day)) expanded = true;
					focusAfter(() => menuOf(entry.id) ?? addOf(entry.day));
				}
			}
		});
	}

	const PAST_ID = 'week-past-days';
	const addBox =
		'flex w-full items-center justify-center gap-1.5 rounded-lg border-[1.5px] border-dashed border-handle px-3.5 text-body-sm font-semibold whitespace-nowrap text-text-muted transition hover:border-primary hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
</script>

{#snippet zone(key: string, layout: 'column' | 'row' | 'chip', label: string)}
	<EntryZone
		items={items(key)}
		{layout}
		{label}
		{today}
		dragging={dragged !== null}
		onconsider={(event) => consider(key, event)}
		onfinalize={(event) => finalize(key, event)}
		onmove={(entry) => {
			moving = entry;
			moveOpen = true;
		}}
		onservings={servings}
		onremove={remove}
	>
		{#snippet empty()}
			{#if key === TRAY}
				<p class="flex h-full items-center text-caption text-text-muted">· {m.plan_tray_empty()}</p>
			{:else}
				<button
					type="button"
					aria-label={m.plan_day_add({ day: dayNames(key, locale).full })}
					data-day-add={key}
					aria-haspopup="dialog"
					onclick={() => onadd(key)}
					class="{addBox} {layout === 'column' ? 'h-10' : 'h-full'}"
				>
					<Plus class="size-4 shrink-0" aria-hidden="true" />
					<!-- Too narrow for the word (a long day name on a small phone), the "+" says it. -->
					<span class="hidden @[7.5rem]:inline" aria-hidden="true">{m.week_add()}</span>
				</button>
			{/if}
		{/snippet}
	</EntryZone>
{/snippet}

<div class="space-y-4">
	<TrayStrip empty={zones[TRAY].length === 0}>
		{@render zone(TRAY, 'chip', m.plan_tray())}
	</TrayStrip>

	{#if !narrow.current}
		<DayStrip bind:this={strip} {days} free={dragged !== null} bind:start {onat} {count} {onsettle}>
			{#snippet column(day)}
				<DayColumn
					{day}
					today={day === today}
					past={day < today}
					empty={items(day).length === 0}
					onadd={() => onadd(day)}
				>
					{@render zone(day, 'column', dayNames(day, locale).full)}
				</DayColumn>
			{/snippet}
		</DayStrip>
	{:else}
		<div class="flex flex-col gap-2.5">
			{#if past.length > 0}
				<PastDays days={past} count={pastCount} bind:expanded controls={PAST_ID} />
			{/if}
			<div id={PAST_ID} class="contents">
				{#each days as day (day)}
					{#if expanded || !past.includes(day)}
						<DayRow
							{day}
							today={day === today}
							empty={items(day).length === 0}
							onadd={() => onadd(day)}
						>
							{@render zone(day, 'row', dayNames(day, locale).full)}
						</DayRow>
					{/if}
				{/each}
			</div>
		</div>
	{/if}
</div>

<MoveSheet
	bind:open={moveOpen}
	entry={moving}
	monday={moving?.day ? mondayOf(moving.day) : narrow.current ? monday : mondayOf(start)}
	{today}
	onpick={moved}
	onCloseAutoFocus={(event) => {
		if (!afterMove) return;
		event.preventDefault();
		focusAfter(afterMove);
		afterMove = null;
	}}
/>
