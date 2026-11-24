<script lang="ts">
	import CalendarPlus from '@lucide/svelte/icons/calendar-plus';
	import { getLocalTimeZone, today as todayIn } from '@internationalized/date';
	import { Dialog } from 'bits-ui';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { isSignedOut } from '#lib/api/client.js';
	import {
		addPlanEntry,
		getPlan,
		listPlans,
		type PlanEntry,
		type PlanSummary
	} from '#lib/api/mealplan.js';
	import { imageUrl } from '#lib/api/recipes.js';
	import { initialFor, tintFor } from '#lib/recipe/placeholder.js';
	import { narrow } from '#lib/ui/narrow.js';
	import NotebookPen from '@lucide/svelte/icons/notebook-pen';
	import ServingsStepper from '#lib/components/recipe/ServingsStepper.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import ResponsiveDialog from '#lib/components/ui/ResponsiveDialog.svelte';
	import Select from '#lib/components/ui/Select.svelte';
	import { TRAY } from '#lib/mealplan/entries.js';
	import { dayNames, mondayOf, shiftWeek, weekDays } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { clampServings } from '#lib/recipe/servings.js';
	import { lastPlan, rememberPlan } from '#lib/shell.svelte.js';
	import { planHref } from '#lib/mealplan/href.js';

	/**
	 * "Add to plan" from a recipe page: a day of this or next week (today
	 * picked and marked) or Someday, the plan (the one opened last, shown
	 * only when there are several), and the servings the page shows. From
	 * md a 720px dialog: the recipe's picture beside the title, the two weeks
	 * as a calendar of date tiles with the meals the plan already has on each
	 * day, and plan and servings side by side; on a phone a sheet of cards:
	 * the photo as a banner, each week a card of seven days with a dot per meal.
	 */
	let {
		open = $bindable(false),
		recipeId,
		title,
		servings,
		cover = null
	}: {
		open?: boolean;
		recipeId: string;
		title: string;
		/** The servings the page shows; the dialog starts there. */
		servings: number;
		/** The recipe's cover image id. */
		cover?: string | null;
	} = $props();

	// What the chosen plan already holds on each day and on the tray, read on
	// opening and again for another plan: a dot per meal on a phone, from md
	// the meals' pictures, a count sticker and their titles.
	const wide = $derived(!narrow.current);
	let planned = $state<PlanEntry[]>([]);
	const onDay = (day: string) => planned.filter((e) => e.day === day);
	const waiting = $derived(planned.filter((e) => e.day === null));
	const titles = (meals: PlanEntry[]) => meals.map((e) => e.title).join(', ');
	/** A day's or the tray's radio name: the day, today, and what is already there. */
	function dayLabel(value: string, meals: PlanEntry[]) {
		if (value === TRAY) {
			return [
				m.plan_tray(),
				meals.length > 0 ? m.plan_add_tray_label({ meals: titles(meals) }) : ''
			]
				.filter(Boolean)
				.join(', ');
		}
		return [
			dayNames(value, locale).full,
			value === today ? m.week_today() : '',
			meals.length > 0 ? m.plan_add_day_planned({ meals: titles(meals) }) : ''
		]
			.filter(Boolean)
			.join(', ');
	}
	/** "Hier wartet schon Kuchen" / "… und 2 weitere": the tray, said kindly. */
	const waitingText = $derived(
		waiting.length === 0
			? m.plan_add_tray_free()
			: waiting.length === 1
				? m.plan_add_tray_waiting_one({ title: waiting[0].title })
				: m.plan_add_tray_waiting({ title: waiting[0].title, count: waiting.length - 1 })
	);
	let occ = 0;
	$effect(() => {
		if (!open || !planId || weeks.length < 2) return;
		const mine = ++occ;
		// Another plan's meals never stand in while this one's load.
		planned = [];
		const range = { from: weeks[0][0], to: weeks[1][6] };
		getPlan(planId, range)
			.then((p) => {
				if (mine === occ) planned = p.entries;
			})
			.catch(() => {
				if (mine === occ) planned = [];
			});
	});

	const locale = getLocale();

	let today = $state('');
	let weeks = $state<string[][]>([]);
	let picked = $state(TRAY);
	let chosen = $state(1);
	let plans = $state<PlanSummary[]>([]);
	let planId = $state('');
	let plansFailed = $state(false);
	let saving = $state(false);

	const planName = (p: PlanSummary) => p.name ?? m.plan_personal();
	/** "Mittwoch" this week; a later one also says its date, "Mittwoch, 25. Nov.". */
	function dayName(day: string) {
		const names = dayNames(day, locale);
		return weeks[0]?.includes(day) ? names.long : `${names.long}, ${names.date}`;
	}

	// A fresh start on every opening: today, the page's servings, the plans as they are now.
	$effect(() => {
		if (!open) return;
		untrack(start);
	});

	function start() {
		today = todayIn(getLocalTimeZone()).toString();
		const monday = mondayOf(today);
		weeks = [weekDays(monday), weekDays(shiftWeek(monday, 1))];
		picked = today;
		chosen = clampServings(servings);
		void loadPlans();
	}

	let req = 0;
	async function loadPlans() {
		const mine = ++req;
		plansFailed = false;
		try {
			const all = (await listPlans()).plans;
			if (mine !== req) return;
			plans = all;
			const last = lastPlan();
			planId = (all.find((p) => p.id === last) ?? all.find((p) => p.personal) ?? all[0])?.id ?? '';
		} catch (error) {
			if (mine !== req) return;
			plansFailed = true;
			if (!isSignedOut(error)) toast.error(m.plan_plans_load_error());
		}
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!planId || saving) return;
		saving = true;
		const day = picked === TRAY ? null : picked;
		const monday = mondayOf(day ?? today);
		const id = planId;
		try {
			await addPlanEntry(
				id,
				{ day, recipeId, servings: chosen },
				{ from: monday, to: weekDays(monday)[6] }
			);
			rememberPlan(id);
			open = false;
			// This week's plan page has no `from`: passing it would only redirect.
			const href = planHref(id, monday === mondayOf(today) ? undefined : monday);
			toast.success(
				day ? m.plan_added({ title, day: dayName(day) }) : m.plan_added_tray({ title }),
				{
					action: {
						label: m.plan_add_open_plan(),
						onClick: () => goto(href)
					}
				}
			);
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.plan_add_error());
		} finally {
			saving = false;
		}
	}

	const dot = 'size-1.5 rounded-full bg-primary group-has-checked/pick:bg-primary-foreground/70';
	const label = 'text-label font-bold tracking-[0.08em] text-text-muted uppercase';
</script>

<!-- How many meals a desktop tile or the tray already holds: a sticker on
     the corner, cut out of the ground around it; it turns light on
     the picked one. -->
{#snippet badge(count: number, past: boolean)}
	<span
		aria-hidden="true"
		class="pointer-events-none absolute -top-1.5 -right-1.5 z-20 flex h-5 min-w-5 items-center justify-center rounded-pill bg-primary px-1.5 text-label font-bold text-primary-foreground tabular-nums ring-2 ring-surface transition peer-checked:bg-surface-elevated peer-checked:text-primary {past
			? 'opacity-60'
			: ''}">{count}</span
	>
{/snippet}

{#snippet thumbOf(entry: PlanEntry, size: string, ring = 'ring-surface-elevated')}
	{#if entry.recipeId && entry.photo}
		<img
			src={imageUrl(entry.recipeId, entry.photo, 'thumb')}
			alt=""
			class="{size} shrink-0 rounded-md object-cover ring-2 {ring}"
		/>
	{:else if entry.recipeId === null && entry.servings === null}
		<span
			class="{size} flex shrink-0 items-center justify-center rounded-md bg-background text-text-muted ring-2 {ring}"
			><NotebookPen class="size-3" /></span
		>
	{:else}
		<span
			class="{size} flex shrink-0 items-center justify-center rounded-md font-display text-label text-primary italic ring-2 {ring} {tintFor(
				entry.recipeId ?? entry.id
			)}">{initialFor(entry.title)}</span
		>
	{/if}
{/snippet}

<!-- What a day or the tray already holds, over it on hover and on keyboard
     focus (the radio is its peer); a touch screen reads it in the line under
     the calendar instead, and a screen reader in the radio's name. -->
{#snippet peek(meals: PlanEntry[], heading: string, align: string)}
	<span
		aria-hidden="true"
		class="pointer-events-none invisible absolute z-20 w-64 rounded-xl border border-popover-border bg-popover p-3 text-text opacity-0 shadow-dialog transition duration-150 peer-focus-visible:visible peer-focus-visible:translate-y-0 peer-focus-visible:opacity-100 motion-reduce:transition-none [@media(hover:hover)]:peer-hover:visible [@media(hover:hover)]:peer-hover:translate-y-0 [@media(hover:hover)]:peer-hover:opacity-100 {align}"
	>
		<span class="mb-2 block {label}">{heading}</span>
		<span class="flex flex-col gap-2">
			{#each meals as meal (meal.id)}
				<span class="flex min-w-0 items-center gap-2.5">
					{@render thumbOf(meal, 'size-9', 'ring-popover')}
					<span class="min-w-0 truncate text-body-sm font-semibold">{meal.title}</span>
				</span>
			{/each}
		</span>
	</span>
{/snippet}

{#snippet tile(value: string, column: number, first: boolean)}
	{@const names = dayNames(value, locale)}
	{@const past = value < today}
	{@const meals = onDay(value)}
	<label class="group/pick relative block">
		<input
			type="radio"
			name="plan-from-recipe-day"
			{value}
			bind:group={picked}
			aria-label={dayLabel(value, meals)}
			class="peer absolute inset-0 z-10 cursor-pointer appearance-none rounded-xl opacity-0"
		/>
		<span
			aria-hidden="true"
			class="flex flex-col rounded-xl border bg-surface-elevated px-2.5 py-2 text-text transition peer-checked:border-primary peer-checked:bg-primary peer-checked:text-primary-foreground peer-hover:border-primary/60 peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-primary {value ===
			today
				? 'border-primary/50'
				: 'border-border'} h-21 {past ? 'opacity-55' : ''}"
		>
			<span
				class="font-display text-heading leading-none font-medium whitespace-nowrap tabular-nums"
				>{names.number}{#if value === today}<span
						class="ml-1 inline-block size-1.5 rounded-full bg-current align-middle"
					></span>{/if}</span
			>
			<span class="mt-auto flex min-h-6 items-center">
				{#if meals.length > 0}
					<span class="flex -space-x-1.5">
						{#each meals.slice(0, 3) as meal (meal.id)}
							{@render thumbOf(meal, 'size-6')}
						{/each}
					</span>
				{:else if value === today}
					<span
						class="text-micro font-bold tracking-[0.06em] uppercase group-has-checked/pick:hidden"
						>{m.week_today()}</span
					>
				{/if}
				<!-- Room for one more beside three: a fourth would leave the tile. -->
				{#if meals.length < 3}{@render arrive('size-6', meals.length > 0)}{/if}
			</span>
		</span>
		{#if meals.length > 0}{@render badge(meals.length, past)}{/if}
		{#if meals.length > 0}
			<!-- The first week's opens downward: above it the scroller would cut it. -->
			{@render peek(
				meals,
				names.full,
				`${column < 4 ? 'left-0' : 'right-0'} ${first ? 'top-full mt-2 -translate-y-1' : 'bottom-full mb-2 translate-y-1'}`
			)}
		{/if}
	</label>
{/snippet}

<!-- This recipe, joining what a day or the tray already holds once it is
     picked: it springs in from nothing on top of the others (the label is
     `group/pick`, so `has-checked` reaches it), a preview of where the
     button will put it. Reduced motion shows it at once. -->
{#snippet arrive(size: string, join: boolean)}
	<span
		aria-hidden="true"
		class="inline-flex max-w-0 shrink-0 scale-50 -rotate-12 opacity-0 transition-[max-width,margin,scale,rotate,opacity] duration-300 ease-spring group-has-checked/pick:max-w-12 group-has-checked/pick:scale-100 group-has-checked/pick:rotate-3 group-has-checked/pick:opacity-100 motion-reduce:transition-none {join
			? 'group-has-checked/pick:-ml-3'
			: ''}"
	>
		{#if cover}
			<img
				src={imageUrl(recipeId, cover, 'thumb')}
				alt=""
				class="{size} shrink-0 rounded-md object-cover shadow-card ring-2 ring-surface-elevated"
			/>
		{:else}
			<span
				class="{size} flex shrink-0 items-center justify-center rounded-md font-display text-label text-primary italic shadow-card ring-2 ring-surface-elevated {tintFor(
					recipeId
				)}">{initialFor(title)}</span
			>
		{/if}
	</span>
{/snippet}

{#snippet trayTile()}
	<label class="group/pick relative mt-2 block">
		<input
			type="radio"
			name="plan-from-recipe-day"
			value={TRAY}
			bind:group={picked}
			aria-label={dayLabel(TRAY, waiting)}
			class="peer absolute inset-0 z-10 cursor-pointer appearance-none rounded-xl opacity-0"
		/>
		<span
			aria-hidden="true"
			class="grid min-h-16 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 rounded-xl border border-dashed border-handle bg-surface-elevated py-2.5 pr-3 pl-4 text-text transition peer-checked:border-solid peer-checked:border-primary peer-checked:bg-primary peer-checked:text-primary-foreground peer-hover:border-primary/60 peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-primary"
		>
			<span class="col-start-1 row-start-1 min-w-0 self-end font-display text-card-sm font-medium"
				>{m.plan_tray()}</span
			>
			<span class="col-start-1 row-start-2 min-w-0 self-start">
				<span class="mt-0.5 block text-caption opacity-75 group-has-checked/pick:hidden"
					><span class="line-clamp-2">{waitingText}</span></span
				>
				<span class="mt-0.5 hidden text-caption group-has-checked/pick:block"
					><span class="line-clamp-2"
						>{waiting.length > 1
							? m.plan_add_tray_joins({ title, count: waiting.length })
							: waiting.length === 1
								? m.plan_add_tray_joins_one({ title, other: waiting[0].title })
								: m.plan_add_tray_joins_alone({ title })}</span
					></span
				>
			</span>
			<!-- The waiting meals as a small fanned stack, the first on top, and
			     this recipe landing on it once the tray is picked. -->
			<span class="col-start-2 row-span-2 row-start-1 flex shrink-0 items-center pr-1">
				{#each waiting.slice(0, 3).reverse() as meal, i (meal.id)}
					<span
						class="-ml-3 first:ml-0 {['-rotate-6', 'rotate-3', '-rotate-2'][
							(Math.min(waiting.length, 3) - 1 - i) % 3
						]}">{@render thumbOf(meal, 'size-10')}</span
					>
				{/each}
				{@render arrive('size-10', waiting.length > 0)}
			</span>
		</span>
		{#if waiting.length > 0}{@render badge(waiting.length, false)}{/if}
		{#if waiting.length > 0}
			{@render peek(waiting, m.plan_add_tray_peek(), 'right-0 bottom-full mb-2 translate-y-1')}
		{/if}
	</label>
{/snippet}

<!-- A phone's day: weekday, date, and a dot per meal the plan already has
     there (no pictures: the tray and every day read alike); picked it fills,
     and one more dot springs in for this recipe. Today sits on the accent. -->
{#snippet cell(value: string)}
	{@const names = dayNames(value, locale)}
	{@const meals = onDay(value)}
	<label class="group/pick relative block min-w-0">
		<input
			type="radio"
			name="plan-from-recipe-day"
			{value}
			bind:group={picked}
			aria-label={dayLabel(value, meals)}
			class="peer absolute inset-0 z-10 cursor-pointer appearance-none rounded-xl opacity-0"
		/>
		<span
			aria-hidden="true"
			class="flex h-16 flex-col items-center rounded-xl pt-1.5 pb-1.5 transition duration-200 peer-checked:bg-primary peer-checked:text-primary-foreground peer-checked:shadow-cta peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-primary {value ===
			today
				? 'bg-accent text-accent-foreground'
				: 'text-text'} {value < today ? 'opacity-45' : ''}"
		>
			<span class="text-micro font-semibold opacity-75">{names.weekday}</span>
			<span class="mt-0.5 font-display text-card leading-none font-medium tabular-nums"
				>{names.number}</span
			>
			<span class="mt-auto flex h-5 items-center justify-center">
				<!-- Up to three dots; from four on one dot and the count, so a busy
				     day never runs out of its cell, with the arrive dot beside either. -->
				<span class="flex items-center gap-0.5">
					{#if meals.length > 3}
						<span class={dot}></span>
						<span
							class="text-micro leading-none font-bold text-primary tabular-nums group-has-checked/pick:text-primary-foreground"
							>{meals.length}</span
						>
					{:else}
						{#each meals as meal (meal.id)}
							<span class={dot}></span>
						{/each}
					{/if}
					<span
						class="hidden size-1.5 rounded-full bg-primary-foreground group-has-checked/pick:block group-has-checked/pick:animate-[pop_300ms_var(--ease-spring)_both] motion-reduce:animate-none"
					></span>
				</span>
			</span>
		</span>
	</label>
{/snippet}

<!-- A phone's tray, dashed like the desktop's: its name, how many wait. -->
{#snippet trayCard()}
	<label class="group/pick relative block">
		<input
			type="radio"
			name="plan-from-recipe-day"
			value={TRAY}
			bind:group={picked}
			aria-label={dayLabel(TRAY, waiting)}
			class="peer absolute inset-0 z-10 cursor-pointer appearance-none rounded-2xl opacity-0"
		/>
		<span
			aria-hidden="true"
			class="flex h-14 items-center justify-between gap-3 rounded-2xl border-2 border-dashed border-handle px-4 text-text transition duration-200 peer-checked:border-solid peer-checked:border-primary peer-checked:bg-primary peer-checked:text-primary-foreground peer-checked:shadow-cta peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-primary"
		>
			<span class="font-display text-card-sm font-medium">{m.plan_tray()}</span>
			<span class="flex items-center gap-2 text-caption opacity-80">
				{#if waiting.length > 0}
					<span class="flex gap-0.5">
						{#each waiting.slice(0, 4) as meal (meal.id)}
							<span class={dot}></span>
						{/each}
					</span>
				{/if}
				{waiting.length === 0
					? m.plan_add_tray_count_none()
					: waiting.length === 1
						? m.plan_add_tray_count_one()
						: m.plan_add_tray_count({ count: waiting.length })}
			</span>
		</span>
	</label>
{/snippet}

{#snippet calendar()}
	<fieldset class="min-w-0">
		<legend class="sr-only">{m.plan_add_day()}</legend>
		<div class="grid grid-cols-7 gap-2" aria-hidden="true">
			{#each weeks[0] ?? [] as day (day)}
				<span class="text-center {label}">{dayNames(day, locale).weekday}</span>
			{/each}
		</div>
		<!-- One calendar of two weeks under one row of weekday names: today's
		     outline and the dates say which week is which, no headings needed. -->
		<div class="mt-2 grid grid-cols-7 gap-2">
			{#each weeks as days, w (days[0])}
				{#each days as day, d (day)}
					{@render tile(day, d, w === 0)}
				{/each}
			{/each}
		</div>
		{@render trayTile()}
		{@render summary()}
	</fieldset>
{/snippet}

<!-- The picked day's meals in words, for touch and for anyone who did not
     hover: one line, its room kept so nothing below jumps. The tray's tile
     says its own. -->
{#snippet summary()}
	<p class="mt-2 min-h-5 truncate text-caption text-text-muted" aria-live="polite">
		{#if picked !== TRAY && onDay(picked).length > 0}
			{m.plan_add_day_has({
				day: dayNames(picked, locale).long,
				meals: titles(onDay(picked))
			})}
		{/if}
	</p>
{/snippet}

{#snippet settings()}
	<!-- Labels on one line, controls centered on the next, whatever their heights. -->
	<div class="grid grid-cols-2 items-center gap-x-4 gap-y-1.5">
		{#if plans.length > 1}
			<span class="row-start-1 {label}">{m.plan_picker()}</span>
			<Select
				bind:value={planId}
				options={plans.map((p) => ({ value: p.id, label: planName(p) }))}
				label={m.plan_picker()}
				class="row-start-2 w-full min-w-0 justify-between bg-surface-elevated"
			/>
		{/if}
		<span class="col-start-2 row-start-1 justify-self-end {label}">{m.add_to_list_servings()}</span>
		<ServingsStepper
			value={chosen}
			onchange={(next) => (chosen = clampServings(next))}
			class="col-start-2 row-start-2 justify-self-end"
		/>
		{#if plansFailed}
			<div class="col-span-2">{@render retry()}</div>
		{/if}
	</div>
{/snippet}

{#snippet retry()}
	<button
		type="button"
		onclick={loadPlans}
		class="flex min-h-11 w-full items-center justify-between rounded-md border border-border bg-surface-elevated px-4 py-1.5 text-left text-caption text-text-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
	>
		<span class="min-w-0 leading-tight">{m.plan_plans_load_error()}</span>
		<span class="shrink-0 pl-3 font-semibold whitespace-nowrap text-primary"
			>{m.add_to_list_retry()}</span
		>
	</button>
{/snippet}

{#snippet picture()}
	{#if cover}
		<img
			src={imageUrl(recipeId, cover, 'detail')}
			alt=""
			class="size-16 shrink-0 rounded-xl object-cover shadow-card"
		/>
	{:else}
		<span
			aria-hidden="true"
			class="flex size-16 shrink-0 items-center justify-center rounded-xl font-display text-heading font-medium text-primary italic shadow-card {tintFor(
				recipeId
			)}">{initialFor(title)}</span
		>
	{/if}
{/snippet}

{#snippet wideBody()}
	<div class="flex items-center gap-4">
		{@render picture()}
		<div class="min-w-0">
			<Dialog.Title class="font-display text-heading font-medium"
				>{m.plan_add_from_recipe()}</Dialog.Title
			>
			<p class="mt-0.5 truncate text-body-sm text-text-muted">{title}</p>
		</div>
	</div>
	<div class="mt-6">{@render calendar()}</div>
	<div class="mt-3">{@render settings()}</div>
{/snippet}

{#snippet body()}
	{#if wide}
		{@render wideBody()}
	{:else}
		<!-- The recipe's photo as the sheet's banner, the title on it. -->
		<div class="relative -mx-1 overflow-hidden rounded-2xl shadow-cover">
			{#if cover}
				<img src={imageUrl(recipeId, cover, 'detail')} alt="" class="h-20 w-full object-cover" />
			{:else}
				<span class="block h-20 w-full {tintFor(recipeId)}"></span>
			{/if}
			<div
				class="absolute inset-0 flex flex-col justify-end bg-linear-to-t from-lightbox/75 via-lightbox/20 to-transparent px-4 pb-3 text-lightbox-foreground"
			>
				<Dialog.Title class="font-display text-heading leading-tight font-medium"
					>{m.plan_add_from_recipe()}</Dialog.Title
				>
				<p class="truncate text-caption opacity-90">{title}</p>
			</div>
		</div>
		<fieldset class="mt-4 flex min-w-0 flex-col gap-2.5">
			<legend class="sr-only">{m.plan_add_day()}</legend>
			{#each weeks as days, w (days[0])}
				{@const count = days.reduce((n, day) => n + onDay(day).length, 0)}
				<div class="rounded-2xl bg-surface-elevated p-1.5 shadow-card">
					<div class="flex items-baseline justify-between px-2 pt-0.5 pb-1" aria-hidden="true">
						<span class="text-caption font-semibold">{w === 0 ? m.week_this() : m.week_next()}</span
						>
						<span class="text-micro text-text-muted"
							>{count > 0 ? m.plan_add_week_planned({ count }) : m.plan_add_week_free()}</span
						>
					</div>
					<div class="grid grid-cols-7 gap-0.5">
						{#each days as day (day)}
							{@render cell(day)}
						{/each}
					</div>
				</div>
			{/each}
			{@render trayCard()}
		</fieldset>
		<!-- Plan and servings as one settings card: labels left, the controls
		     in one right column, so the plan field is never narrower than the stepper. -->
		<div
			class="mt-2.5 grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2 rounded-2xl bg-surface-elevated py-0.5 pr-1.5 pl-3.5 shadow-card min-[360px]:gap-x-3 min-[360px]:pl-4"
		>
			{#if plans.length > 1}
				<span class="text-caption font-semibold min-[360px]:text-body-sm">{m.plan_picker()}</span>
				<Select
					bind:value={planId}
					options={plans.map((p) => ({ value: p.id, label: planName(p) }))}
					label={m.plan_picker()}
					variant="pill"
					class="my-2 h-10 w-full max-w-[55vw] min-w-0 justify-between bg-background pl-4 text-body-sm"
				/>
				<span class="col-span-2 h-px bg-border" aria-hidden="true"></span>
			{/if}
			<span class="text-caption font-semibold min-[360px]:text-body-sm"
				>{m.add_to_list_servings()}</span
			>
			<ServingsStepper
				value={chosen}
				onchange={(next) => (chosen = clampServings(next))}
				class="my-2 justify-self-end"
			/>
			{#if plansFailed}
				<div class="col-span-2 pb-3">{@render retry()}</div>
			{/if}
		</div>
	{/if}
{/snippet}

{#snippet footer()}
	<Button variant="secondary" class="px-3 md:px-5" onclick={() => (open = false)}
		>{m.common_cancel()}</Button
	>
	<Button
		type="submit"
		busy={saving}
		disabled={!planId || saving}
		class="min-w-0 grow px-3 whitespace-nowrap md:grow-0 md:px-5"
	>
		<CalendarPlus class="size-4 shrink-0" aria-hidden="true" />
		{m.plan_add_from_recipe()}
	</Button>
{/snippet}

<ResponsiveDialog
	bind:open
	{wide}
	closeLabel={m.common_cancel()}
	onsubmit={submit}
	footerClass="flex gap-2 px-4 pt-3.5 min-[340px]:px-5"
	{body}
	{footer}
/>
