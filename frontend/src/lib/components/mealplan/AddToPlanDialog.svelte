<script lang="ts">
	import CalendarPlus from '@lucide/svelte/icons/calendar-plus';
	import { getLocalTimeZone, today as todayIn } from '@internationalized/date';
	import { Dialog } from 'bits-ui';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { isSignedOut } from '#lib/api/client.js';
	import { addPlanEntry, listPlans, type PlanSummary } from '#lib/api/mealplan.js';
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

	/**
	 * "Add to plan" from a recipe page: a day of this or next week (today
	 * picked and marked) or Someday, the plan (the one opened last, shown
	 * only when there are several), and the servings the page shows.
	 */
	let {
		open = $bindable(false),
		recipeId,
		title,
		servings
	}: {
		open?: boolean;
		recipeId: string;
		title: string;
		/** The servings the page shows; the dialog starts there. */
		servings: number;
	} = $props();

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
			const href = `/week?${new URLSearchParams({ plan: id, from: monday })}` as const;
			toast.success(
				day ? m.plan_added({ title, day: dayName(day) }) : m.plan_added_tray({ title }),
				{
					action: {
						label: m.plan_add_open_week(),
						onClick: () => goto(resolve(href))
					}
				}
			);
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.plan_add_error());
		} finally {
			saving = false;
		}
	}

	const label = 'text-label font-bold tracking-[0.08em] text-text-muted uppercase';
	const chip =
		'flex h-11 cursor-pointer items-center justify-center gap-1 rounded-pill border border-border bg-surface-elevated px-1.5 text-body-sm whitespace-nowrap text-text transition peer-checked:border-primary peer-checked:bg-primary peer-checked:text-primary-foreground peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-primary md:h-10';
</script>

{#snippet option(value: string, past: boolean)}
	{@const names = value === TRAY ? null : dayNames(value, locale)}
	<label class="relative {value === TRAY ? 'col-span-2' : ''}">
		<input
			type="radio"
			name="plan-from-recipe-day"
			{value}
			bind:group={picked}
			aria-label={names
				? value === today
					? `${names.full}, ${m.week_today()}`
					: names.full
				: m.plan_tray()}
			class="peer absolute inset-0 z-10 cursor-pointer appearance-none rounded-pill opacity-0"
		/>
		<span class="{chip} {past ? 'text-text-muted' : ''}" aria-hidden="true">
			{#if names}
				<span class="font-semibold">{names.weekday}</span>
				<span class="tabular-nums opacity-80">{names.number}</span>
				{#if value === today}<span class="ml-0.5 size-1.5 rounded-full bg-current"></span>{/if}
			{:else}
				<span class="font-semibold">{m.plan_tray()}</span>
			{/if}
		</span>
	</label>
{/snippet}

{#snippet body()}
	<Dialog.Title class="font-display text-heading font-medium"
		>{m.plan_add_from_recipe()}</Dialog.Title
	>
	<p class="mt-1 text-caption text-text-muted">{title}</p>
	<fieldset class="mt-5">
		<legend class="sr-only">{m.plan_add_day()}</legend>
		{#each weeks as days, w (days[0])}
			<p class="mb-2 {w > 0 ? 'mt-4' : ''} {label}" aria-hidden="true">
				{w === 0 ? m.week_this() : m.week_next()}
			</p>
			<!-- Four across on a phone, where seven would be narrower than a finger; a row a week from md. -->
			<div class="grid grid-cols-4 gap-1.5 md:grid-cols-7">
				{#each days as day (day)}
					{@render option(day, day < today)}
				{/each}
			</div>
		{/each}
		<div class="mt-3 grid grid-cols-4 gap-1.5 md:grid-cols-7">
			{@render option(TRAY, false)}
		</div>
	</fieldset>
	<div class="mt-5 space-y-4">
		{#if plans.length > 1}
			<div class="flex items-center justify-between gap-3">
				<span class={label}>{m.plan_picker()}</span>
				<Select
					bind:value={planId}
					options={plans.map((p) => ({ value: p.id, label: planName(p) }))}
					label={m.plan_picker()}
					class="max-w-[60%] min-w-0 justify-between bg-surface-elevated"
				/>
			</div>
		{/if}
		<div class="flex items-center justify-between">
			<span class={label}>{m.add_to_list_servings()}</span>
			<ServingsStepper value={chosen} onchange={(next) => (chosen = clampServings(next))} />
		</div>
		{#if plansFailed}
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
		{/if}
	</div>
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
	closeLabel={m.common_cancel()}
	onsubmit={submit}
	footerClass="flex gap-2 px-4 pt-3.5 min-[340px]:px-5"
	{body}
	{footer}
/>
