<script lang="ts">
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Plus from '@lucide/svelte/icons/plus';
	import Share2 from '@lucide/svelte/icons/share-2';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import { onMount, tick } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { toast } from 'svelte-sonner';
	import { afterNavigate, goto, replaceState } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import {
		addPlanEntry,
		deletePlan,
		invitePlan,
		leavePlan,
		removePlanMember,
		renamePlan,
		type PlanEntry
	} from '#lib/api/mealplan.js';
	import { session } from '#lib/auth.svelte.js';
	import AddEntrySheet from '#lib/components/mealplan/AddEntrySheet.svelte';
	import PlanMenu from '#lib/components/mealplan/PlanMenu.svelte';
	import PeopleSheet from '#lib/components/shopping/PeopleSheet.svelte';
	import PeopleStack from '#lib/components/shopping/PeopleStack.svelte';
	import ShareDialog from '#lib/components/shopping/ShareDialog.svelte';
	import StampRow from '#lib/components/shopping/StampRow.svelte';
	import ToListDialog from '#lib/components/mealplan/ToListDialog.svelte';
	import WeekView from '#lib/components/mealplan/WeekView.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import IconButton, { iconCircle } from '#lib/components/ui/IconButton.svelte';
	import NoteEmptyState from '#lib/components/ui/NoteEmptyState.svelte';
	import { PlanState } from '#lib/mealplan/plan-state.svelte.js';
	import { mondayOf, pastDays, shiftWeek, weekDays, weekRange } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { planHref } from '#lib/mealplan/href.js';
	import { arrival } from '#lib/mealplan/overview.svelte.js';
	import { setToList } from '#lib/mealplan/to-list.js';
	import { rememberPlan, shell } from '#lib/shell.svelte.js';
	import { listHref } from '#lib/shopping/href.js';
	import { byPlanUse } from '#lib/shopping/plan-ref.js';
	import planArt from '$brand/lockups/rezepte-mascot-shopping.webp?url';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	/**
	 * Whether the entry before this plan in the history is the overview (the
	 * plan was opened from it), so going back to it is a step back, not a
	 * new entry. Paging weeks keeps it.
	 */
	let fromOverview = false;
	afterNavigate(({ from, to }) => {
		// "Plan today" on the overview's card: the add sheet on today, once
		// (after the navigation: the router takes no `replaceState` before).
		if (page.url.searchParams.has('add')) {
			const url = new URL(page.url.href);
			url.searchParams.delete('add');
			replaceState(url, page.state);
			openAdd();
		}
		if (from?.route.id === '/plans/[id]' && from.params?.id === to?.params?.id) return;
		fromOverview = from?.route.id === '/plans';
		// Arriving, the keyboard starts at the plan, not at the logo; the
		// share button keeps it on a plan just made.
		if (from && !page.state.created)
			document.querySelector<HTMLElement>('[data-plan-title]')?.focus({ preventScroll: true });
		jump();
	});

	/**
	 * A stamp's jump (`?day=&entry=`): a phone unfolds the day if it is
	 * folded, the page scrolls the meal (else the day) to the middle,
	 * focuses it and lights it up once. Done once: the address drops the
	 * two, so Back or a reload does not jump again.
	 */
	function jump() {
		const day = page.url.searchParams.get('day');
		const entry = page.url.searchParams.get('entry');
		if (!day && !entry) return;
		const url = new URL(page.url.href);
		url.searchParams.delete('day');
		url.searchParams.delete('entry');
		replaceState(url, page.state);
		if (day && pastDays(monday, data.today).includes(day)) expandedWeek = monday;
		void tick().then(() => {
			const card = entry
				? document.querySelector<HTMLElement>(`[data-entry-id="${CSS.escape(entry)}"]`)
				: null;
			const target =
				card ??
				(day ? document.querySelector<HTMLElement>(`[data-day="${CSS.escape(day)}"]`) : null);
			if (!target) return;
			target.scrollIntoView({ block: 'center', behavior: 'smooth' });
			(card?.querySelector<HTMLElement>('a[href], [data-entry-menu]') ?? target).focus({
				preventScroll: true
			});
			// The card itself lights up: its own background would cover its slot's.
			const lit = card?.firstElementChild ?? target;
			lit.classList.add('motion-safe:animate-flash');
			setTimeout(() => lit.classList.remove('motion-safe:animate-flash'), 1800);
		});
	}

	/** Back to the overview the plan was opened from, else to it in the plan's place. */
	function back(event?: MouseEvent) {
		if (!fromOverview) return false;
		event?.preventDefault();
		history.back();
		return true;
	}

	/**
	 * To the overview once the shown plan is gone for the reader (deleted,
	 * left, removed): Back must never reopen a plan that is no longer there,
	 * so the plan's entry is left (back) or replaced. The overview puts the
	 * focus on its heading, where the toast talks.
	 */
	async function home() {
		arrival.overview = true;
		if (back()) return;
		await goto(resolve('/plans'), { invalidateAll: true, replaceState: true });
	}

	// The plan went away under the page (deleted, or the reader removed):
	// the page says so and goes to the overview.
	function fail(error: unknown, message = m.week_load_error()) {
		if (isSignedOut(error)) return;
		if (error instanceof ApiError && error.status === 404) {
			toast.error(m.plan_gone());
			void home();
			return;
		}
		toast.error(message);
	}
	const failChange = (error: unknown) => fail(error, m.plan_change_error());

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
	/** Every list the shown week and the tray went to, as far as the reader can open them; most used first (`byPlanUse`). */
	const listStamps = $derived(
		byPlanUse(plan.lists).map((l) => ({
			key: l.id,
			kind: 'list' as const,
			label: l.name,
			open: l.openCount,
			href: listHref(l.id)
		}))
	);
	const wide = new MediaQuery('min-width: 1024px');

	$effect(() => rememberPlan(plan.id));

	const owner = $derived(plan.owner.id === session.user?.id);
	const people = $derived([plan.owner, ...plan.members]);
	const planName = $derived(plan.name ?? '');
	const title = $derived(plan.personal ? m.plan_personal() : planName);

	$effect(() => {
		shell.breadcrumb = title;
		shell.parent = { href: resolve('/plans'), label: m.plans_title() };
		return () => {
			shell.breadcrumb = undefined;
			shell.parent = undefined;
		};
	});
	let shareOpen = $state(false);
	let peopleOpen = $state(false);

	// A plan just made (`/plans/new`) offers its sharing first, once on
	// arrival: the plan itself changes with every edit and refetch.
	onMount(() => {
		if (page.state.created && !plan.personal)
			document.querySelector<HTMLElement>('[data-plan-share]')?.focus();
	});

	async function removePlan() {
		const name = planName;
		try {
			await deletePlan(plan.id);
			toast.success(m.plan_deleted({ name }));
			await home();
		} catch (error) {
			failChange(error);
		}
	}

	async function leave() {
		const name = planName;
		try {
			await leavePlan(plan.id);
			toast.success(m.plan_left({ name }));
			await home();
		} catch (error) {
			failChange(error);
		}
	}

	/**
	 * Another week of this plan; this week's address carries no `from`. It
	 * replaces the shown week in the history, so Back leaves the plan.
	 */
	function show(from: string) {
		void goto(planHref(plan.id, from === thisWeek ? undefined : from), {
			reset: false,
			replaceState: true
		});
	}

	/** A phone's days gone by are folded; the week they were opened for. */
	let expandedWeek = $state('');

	// The week's action takes the place of New list in the top bar, and of
	// the new-things sheet behind a phone's "+": both open the add sheet, on
	// today in this week, else on the shown week's Monday.
	let adding = $state(false);
	let listing = $state(false);
	/** "Auf eine Liste…" from one entry's ⋯ or its "Auf keiner Liste": the dialog for it alone. */
	let listingOne = $state<PlanEntry | null>(null);
	setToList((entry) => {
		listingOne = entry;
		listing = true;
	});
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

<svelte:head><title>{title} · {weekRange(monday, locale)} · {m.app_name()}</title></svelte:head>

<div class="pt-6 pb-8 md:pt-4">
	<!-- Like a list's head: phone back and the plan's controls above the
	     name; on a desktop the controls beside it, the top bar's breadcrumb
	     naming the way back. -->
	<header class="mb-5 grid grid-cols-[minmax(0,1fr)_auto] items-end gap-x-4">
		<IconButton
			href={resolve('/plans')}
			onclick={back}
			label={m.detail_back()}
			class="justify-self-start max-md:size-11 lg:hidden"
		>
			<ArrowLeft class="size-5" aria-hidden="true" />
		</IconButton>
		<h1
			tabindex="-1"
			data-plan-title
			class="col-span-2 row-start-2 mt-5 min-w-0 font-display text-display-md font-medium wrap-break-word outline-none md:text-display-lg lg:col-span-1 lg:row-start-1 lg:mt-0"
		>
			{title}
		</h1>
		<div class="col-start-2 row-start-1 flex items-center gap-2 lg:mb-1">
			{#if !plan.personal}
				{#if people.length > 1}
					<button
						type="button"
						aria-haspopup="dialog"
						aria-label={m.plan_people_show({ count: people.length })}
						onclick={() => (peopleOpen = true)}
						class="relative rounded-pill focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary max-md:after:absolute max-md:after:inset-x-0 max-md:after:-inset-y-2"
					>
						<PeopleStack {people} />
					</button>
				{/if}
				{#if owner}
					<button
						type="button"
						aria-haspopup="dialog"
						aria-label={m.plan_share()}
						data-plan-share
						onclick={() => (shareOpen = true)}
						class="{iconCircle} bg-surface text-text max-md:size-11"
					>
						<Share2 class="size-5 -translate-x-px" aria-hidden="true" />
					</button>
				{/if}
			{/if}
			<PlanMenu
				personal={plan.personal}
				name={planName}
				{owner}
				onrename={(name) =>
					week.apply(renamePlan(plan.id, name, week.range), undefined, failChange)}
				onshare={() => (shareOpen = true)}
				ondelete={removePlan}
				onleave={leave}
			/>
		</div>
	</header>

	<!-- The week bar: the week and its pager, every list the week went to,
	     and "Put on the list". From md the pager and the button share a row
	     and the lists take the full width below (one line from lg, else two,
	     then "+n"); on a phone all stacked, the button over the full width. -->
	<div class="mb-4 flex flex-col gap-2.5 md:flex-row md:flex-wrap md:items-center md:gap-x-3">
		<div
			class="flex items-center justify-between gap-2 md:mr-1 md:shrink-0 md:justify-start md:gap-4"
		>
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
		{#if plan.lists.length > 0}
			<!-- A row of their own under the date from md: the bar keeps the
			     pager and the button, the lists get the width. -->
			<!-- Below sm the label sits above its row, which gets the full width. -->
			<div
				class="flex min-w-0 items-start gap-x-2 max-sm:flex-col max-sm:gap-y-0.5 md:order-last md:w-full"
			>
				<span class="shrink-0 text-caption leading-6 text-text-muted" aria-hidden="true"
					>{m.plans_card_lists()}</span
				>
				<StampRow
					label={m.link_plan_lists()}
					lines={wide.current ? 1 : 2}
					stamps={listStamps}
					class="w-full flex-1"
				/>
			</div>
		{/if}
		{#if !empty}
			<div class="flex min-w-0 flex-col gap-1 md:ml-auto md:flex-row md:items-center md:gap-3">
				<!-- The count is what the dialog would add for the shown week and the tray. -->
				<Button
					variant="primary"
					aria-haspopup="dialog"
					aria-expanded={listing}
					label={plan.unlisted > 0
						? `${m.plan_to_list()}, ${plan.unlisted === 1 ? m.plan_to_list_count_one() : m.plan_to_list_count({ count: plan.unlisted })}`
						: undefined}
					onclick={() => {
						listingOne = null;
						listing = true;
					}}
					class="shrink-0 shadow-cta max-md:h-11 max-md:w-full md:px-4.5"
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

{#if !plan.personal}
	<PeopleSheet
		bind:open={peopleOpen}
		owner={plan.owner}
		members={plan.members}
		kind="plan"
		onshare={owner ? () => (shareOpen = true) : undefined}
	/>
	{#if owner}
		<ShareDialog
			bind:open={shareOpen}
			title={m.plan_share_title({ name: planName })}
			text={m.plan_share_text()}
			owner={plan.owner}
			members={plan.members}
			pending={plan.pending}
			invite={(userId) => invitePlan(plan.id, userId, week.range)}
			remove={(userId) => removePlanMember(plan.id, userId, week.range)}
			apply={(request, onfail) => week.apply(request, undefined, onfail ?? failChange)}
			ongone={fail}
		/>
	{/if}
{/if}

<ToListDialog bind:open={listing} {week} today={data.today} only={listingOne} />
<AddEntrySheet bind:open={adding} {days} day={addDay} today={data.today} onadd={add} />
