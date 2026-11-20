<script lang="ts">
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Plus from '@lucide/svelte/icons/plus';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import { addPlanEntry } from '#lib/api/mealplan.js';
	import AddEntrySheet from '#lib/components/mealplan/AddEntrySheet.svelte';
	import PlanPicker from '#lib/components/mealplan/PlanPicker.svelte';
	import ToListDialog from '#lib/components/mealplan/ToListDialog.svelte';
	import WeekView from '#lib/components/mealplan/WeekView.svelte';
	import PlaceSwitch from '#lib/components/shell/PlaceSwitch.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import IconButton from '#lib/components/ui/IconButton.svelte';
	import NoteEmptyState from '#lib/components/ui/NoteEmptyState.svelte';
	import { PlanState } from '#lib/mealplan/plan-state.svelte.js';
	import { mondayOf, pastDays, shiftWeek, weekDays, weekRange } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { lastPlan, rememberPlan, shell } from '#lib/shell.svelte.js';
	import { listHref } from '#lib/shopping/href.js';
	import planArt from '$brand/lockups/rezepte-mascot-shopping.webp?url';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// The plan went away under the page (deleted, or the reader removed):
	// the week reopens on the personal plan.
	function fail(error: unknown) {
		if (isSignedOut(error)) return;
		if (error instanceof ApiError && error.status === 404) {
			toast.error(m.week_load_error());
			void goto(resolve('/week'));
			return;
		}
		toast.error(m.week_load_error());
	}

	// Starts as the first load; `connect` follows each later one (another
	// week, another plan), and a member's change refetches the shown week.
	// svelte-ignore state_referenced_locally
	const week = new PlanState(data.plan, fail);
	week.connect(() => data.plan);
	const plan = $derived(week.plan);

	const locale = getLocale();
	const monday = $derived(plan.from);
	const days = $derived(weekDays(monday));
	const thisWeek = $derived(mondayOf(data.today));
	const empty = $derived(plan.entries.length === 0);
	/** The list most of the week went to, as far as the reader can open it. */
	const list = $derived(plan.lists[0]);

	$effect(() => rememberPlan(plan.id));

	/**
	 * Another week or plan. `plan` is left out only for the plan the page
	 * would reopen on anyway, the one remembered last (the shown one); a pick
	 * of another plan always names it, the personal one too.
	 */
	function show(from: string, id = plan.id) {
		const query: [string, string][] = [];
		if (id !== lastPlan()) query.push(['plan', id]);
		if (from !== thisWeek) query.push(['from', from]);
		const qs = new URLSearchParams(query).toString();
		void goto(resolve(qs ? `/week?${qs}` : '/week'), { reset: false });
	}

	/** A phone's days gone by are folded; the week they were opened for. */
	let expandedWeek = $state('');

	// The week's action takes the place of New list in the top bar, and of
	// the new-things sheet behind a phone's "+": both open the add sheet, on
	// today in this week, else on the shown week's Monday.
	let adding = $state(false);
	let listing = $state(false);
	let addDay = $state<string | null>(null);
	function openAdd(day: string | null = days.includes(data.today) ? data.today : monday) {
		addDay = day;
		adding = true;
	}
	$effect(() => {
		shell.actions = addAction;
		shell.add = {
			label: m.week_add(),
			open: () => openAdd(),
			get expanded() {
				return adding;
			}
		};
		return () => {
			shell.actions = undefined;
			shell.add = undefined;
		};
	});

	async function add(input: { day: string | null; recipeId?: string; title?: string }) {
		const done = await week.apply(addPlanEntry(plan.id, input, week.range), undefined, (error) => {
			if (!isSignedOut(error)) toast.error(m.plan_add_error());
		});
		// Planned onto a folded day: open the fold, so it shows where it went.
		if (done && input.day && pastDays(monday, data.today).includes(input.day))
			expandedWeek = monday;
		return done;
	}
</script>

{#snippet addAction()}
	<Button variant="primary" aria-haspopup="dialog" aria-expanded={adding} onclick={() => openAdd()}>
		<Plus class="size-4" aria-hidden="true" />
		{m.week_add()}
	</Button>
{/snippet}

<svelte:head><title>{weekRange(monday, locale)} · {m.app_name()}</title></svelte:head>

<div class="pt-6 pb-8 md:pt-4">
	<PlaceSwitch side="week" />

	<!-- The week bar: the week and its pager, then the plan, the list it went
	     to and "Put on the list". One row on a desktop; on a phone stacked,
	     the button over the full width. -->
	<div
		class="mb-4 flex flex-col gap-2.5 md:flex-row md:flex-wrap md:items-center md:gap-x-3 md:gap-y-2"
	>
		<div class="flex items-center justify-between gap-2 md:mr-1 md:justify-start md:gap-4">
			<h2
				class="min-w-0 font-display text-card-sm font-medium whitespace-nowrap min-[360px]:text-card md:text-heading"
			>
				<span class="max-[1199.98px]:hidden">{weekRange(monday, locale)}</span>
				<span class="min-[1200px]:hidden">{weekRange(monday, locale, 'short')}</span>
			</h2>
			<div class="flex shrink-0 items-center gap-1 md:gap-1.5">
				<IconButton
					label={m.week_previous()}
					class="max-md:size-11"
					onclick={() => show(shiftWeek(monday, -1))}
				>
					<ChevronLeft class="size-4.5" aria-hidden="true" />
				</IconButton>
				<Button
					variant="secondary"
					class="max-md:h-11 max-md:px-3.5"
					onclick={() => show(thisWeek)}
				>
					{m.week_today()}
				</Button>
				<IconButton
					label={m.week_next()}
					class="max-md:size-11"
					onclick={() => show(shiftWeek(monday, 1))}
				>
					<ChevronRight class="size-4.5" aria-hidden="true" />
				</IconButton>
			</div>
		</div>
		{#if data.plans.length > 1}
			<div class="flex max-md:order-first">
				<PlanPicker plans={data.plans} value={plan.id} onchange={(id) => show(monday, id)} />
			</div>
		{/if}
		{#if !empty}
			<div class="flex flex-col gap-1 md:ml-auto md:flex-row md:items-center md:gap-3">
				{#if list}
					<a
						href={listHref(list.id)}
						class="flex min-h-11 items-center gap-1.5 px-0.5 text-body-sm whitespace-nowrap max-md:order-2 md:min-h-10"
					>
						<span class="flex min-w-0 items-center gap-1.5 font-semibold text-success-foreground">
							<ShoppingBasket
								class="size-4 shrink-0 md:max-[1199.98px]:hidden"
								aria-hidden="true"
							/>
							<span class="truncate">{list.name}</span>
						</span>
						<span class="text-text-muted">· {m.list_open({ open: list.openCount })}</span>
						<ChevronRight
							class="size-4 shrink-0 text-text-muted max-md:ml-auto"
							aria-hidden="true"
						/>
					</a>
				{/if}
				<!-- The count is what the dialog would add for the shown week and the tray. -->
				<Button
					variant="primary"
					aria-haspopup="dialog"
					aria-expanded={listing}
					label={plan.unlisted > 0
						? `${m.plan_to_list()}, ${plan.unlisted === 1 ? m.plan_to_list_count_one() : m.plan_to_list_count({ count: plan.unlisted })}`
						: undefined}
					onclick={() => (listing = true)}
					class="shadow-cta max-md:h-11 max-md:w-full md:px-4.5"
				>
					<ShoppingBasket class="size-4" aria-hidden="true" />
					{m.plan_to_list()}
					{#if plan.unlisted > 0}
						<span
							class="inline-flex h-5.5 min-w-5.5 items-center justify-center rounded-full bg-primary-foreground px-1.5 text-caption font-bold text-primary"
							aria-hidden="true">{plan.unlisted}</span
						>
					{/if}
				</Button>
			</div>
		{/if}
	</div>

	{#if empty}
		<NoteEmptyState
			art={planArt}
			title={m.week_empty_title()}
			steps={[
				[m.week_empty_step_plan(), m.week_empty_step_plan_hint()],
				[m.week_empty_step_list(), m.week_empty_step_list_hint()],
				[m.week_empty_step_shop(), m.week_empty_step_shop_hint()]
			]}
		>
			<Button
				variant="primary"
				size="lg"
				aria-haspopup="dialog"
				onclick={() => openAdd()}
				class="group shadow-cta md:h-12"
			>
				<Plus
					aria-hidden="true"
					strokeWidth={2.5}
					class="size-5 transition-transform duration-300 ease-spring group-hover:rotate-90 motion-reduce:transition-none"
				/>
				{m.week_add()}
			</Button>
		</NoteEmptyState>
	{:else}
		<WeekView
			{week}
			today={data.today}
			bind:expanded={() => expandedWeek === monday, (open) => (expandedWeek = open ? monday : '')}
			onadd={openAdd}
		/>
	{/if}
</div>

<ToListDialog bind:open={listing} {week} today={data.today} />
<AddEntrySheet bind:open={adding} {days} day={addDay} today={data.today} onadd={add} />
