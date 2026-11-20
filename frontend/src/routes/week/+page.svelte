<script lang="ts">
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Plus from '@lucide/svelte/icons/plus';
	import Share2 from '@lucide/svelte/icons/share-2';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import { onMount, tick, untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import {
		acceptPlan,
		addPlanEntry,
		deletePlan,
		invitePlan,
		leavePlan,
		removePlanMember,
		renamePlan
	} from '#lib/api/mealplan.js';
	import { session } from '#lib/auth.svelte.js';
	import AddEntrySheet from '#lib/components/mealplan/AddEntrySheet.svelte';
	import PlanMenu from '#lib/components/mealplan/PlanMenu.svelte';
	import PlanPicker from '#lib/components/mealplan/PlanPicker.svelte';
	import InvitationCard from '#lib/components/shopping/InvitationCard.svelte';
	import PeopleSheet from '#lib/components/shopping/PeopleSheet.svelte';
	import PeopleStack from '#lib/components/shopping/PeopleStack.svelte';
	import ShareDialog from '#lib/components/shopping/ShareDialog.svelte';
	import ToListDialog from '#lib/components/mealplan/ToListDialog.svelte';
	import WeekView from '#lib/components/mealplan/WeekView.svelte';
	import PlaceSwitch from '#lib/components/shell/PlaceSwitch.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import IconButton, { iconCircle } from '#lib/components/ui/IconButton.svelte';
	import NoteEmptyState from '#lib/components/ui/NoteEmptyState.svelte';
	import { PlanState } from '#lib/mealplan/plan-state.svelte.js';
	import { mondayOf, pastDays, shiftWeek, weekDays, weekRange } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { holdKeyboard } from '#lib/keyboard.js';
	import { lastPlan, rememberPlan, shell } from '#lib/shell.svelte.js';
	import { listHref } from '#lib/shopping/href.js';
	import { invitations } from '#lib/shopping/invitations.svelte.js';
	import planArt from '$brand/lockups/rezepte-mascot-shopping.webp?url';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let picker = $state<HTMLElement | null>(null);

	/**
	 * Back to the personal plan once the shown one is gone for the reader
	 * (deleted, left, removed): the load falls back to it, since the plan
	 * remembered last is no longer among the reader's. The focus goes to the
	 * picker, which names the plan now shown; the control it was on is gone.
	 */
	async function home() {
		await goto(resolve('/week'), { invalidateAll: true });
		await tick();
		picker?.focus();
	}

	// The plan went away under the page (deleted, or the reader removed):
	// the week says so and reopens on the personal plan.
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
	/** The list most of the week went to, as far as the reader can open it. */
	const list = $derived(plan.lists[0]);

	$effect(() => rememberPlan(plan.id));

	const owner = $derived(plan.owner.id === session.user?.id);
	const people = $derived([plan.owner, ...plan.members]);
	const planName = $derived(plan.name ?? '');
	// The picker names the shown plan as it is now: a rename lands here first.
	const plans = $derived(data.plans.map((p) => (p.id === plan.id ? { ...p, name: plan.name } : p)));
	let shareOpen = $state(false);
	let peopleOpen = $state(false);

	// Invitations to plans wait above the week, as the lists' do above the lists.
	let answered = $state<string[]>([]);
	const invited = $derived(data.invitations.filter((i) => !answered.includes(i.planId)));
	$effect(() => {
		invitations.plans = invited.length;
	});
	// The shell's count moved on its own (the tab came back, an invitation
	// arrived or was withdrawn meanwhile): load the cards it counts.
	$effect(() => {
		if (invitations.plans !== untrack(() => invited.length)) void invalidateAll();
	});

	// A plan just made (`/week/new`) offers its sharing first, once on
	// arrival: the plan itself changes with every edit and refetch.
	onMount(() => {
		if (page.state.created && !plan.personal)
			document.querySelector<HTMLElement>('[data-plan-share]')?.focus();
	});

	async function accept(planId: string) {
		await acceptPlan(planId, week.range);
		rememberPlan(planId);
		show(monday, planId, true);
	}

	function newPlan() {
		// Inside the pick: the draft's name field takes the keyboard over.
		holdKeyboard();
		void goto(resolve('/week/new'));
	}

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
	 * Another week or plan. `plan` is left out only for the plan the page
	 * would reopen on anyway, the one remembered last (the shown one); a pick
	 * of another plan always names it, the personal one too.
	 */
	function show(from: string, id = plan.id, invalidateAll = false) {
		const query: [string, string][] = [];
		if (id !== lastPlan()) query.push(['plan', id]);
		if (from !== thisWeek) query.push(['from', from]);
		const qs = new URLSearchParams(query).toString();
		void goto(resolve(qs ? `/week?${qs}` : '/week'), { reset: false, invalidateAll });
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

	{#if invited.length > 0}
		<div class="mb-6 flex flex-col gap-3">
			{#each invited as invitation (invitation.planId)}
				{@const name = invitation.name ?? ''}
				<InvitationCard
					from={invitation.inviter}
					title={m.plan_invite_title({ name: invitation.inviter.displayName, plan: name })}
					hint={m.plan_invite_hint()}
					gone={m.plan_gone()}
					accept={() => accept(invitation.planId)}
					decline={() => leavePlan(invitation.planId)}
					ondone={() => (answered = [...answered, invitation.planId])}
				/>
			{/each}
		</div>
	{/if}

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
		<!-- Which plan, for a named one who is on it, and its ⋯ (the calendar for every plan). -->
		<div class="flex min-w-0 items-center gap-2 max-md:order-first">
			<PlanPicker
				{plans}
				value={plan.id}
				bind:ref={picker}
				onchange={(id) => show(monday, id)}
				onnew={newPlan}
			/>
			<div class="flex shrink-0 items-center gap-2 max-md:ml-auto">
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
						<!-- Round like the ⋯ beside it, so the week bar keeps to one line on a desktop. -->
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
		</div>
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

<ToListDialog bind:open={listing} {week} today={data.today} />
<AddEntrySheet bind:open={adding} {days} day={addDay} today={data.today} onadd={add} />
