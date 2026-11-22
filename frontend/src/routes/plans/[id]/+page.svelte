<script lang="ts">
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Plus from '@lucide/svelte/icons/plus';
	import Share2 from '@lucide/svelte/icons/share-2';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import { onMount, tick, untrack } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MediaQuery } from 'svelte/reactivity';
	import { toast } from 'svelte-sonner';
	import { getLocalTimeZone, today as todayIn } from '@internationalized/date';
	import { afterNavigate, goto, invalidateAll, replaceState } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { navigating, page } from '$app/state';
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
	import DayTimeline from '#lib/components/mealplan/DayTimeline.svelte';
	import WeekView from '#lib/components/mealplan/WeekView.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import IconButton, { iconCircle } from '#lib/components/ui/IconButton.svelte';
	import NoteEmptyState from '#lib/components/ui/NoteEmptyState.svelte';
	import { PlanState } from '#lib/mealplan/plan-state.svelte.js';
	import { countUnlisted, listsOf, shownEntries } from '#lib/mealplan/entries.js';
	import {
		aroundToday,
		dayNumber,
		dayRange,
		daysFrom,
		mondayOf,
		pastDays,
		shiftDay,
		shiftWeek,
		weekDays,
		weekFromNow,
		windowFromNow
	} from '#lib/mealplan/week.js';
	import { narrow } from '#lib/ui/narrow.js';
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
	afterNavigate(({ from, to, type }) => {
		// "Plan today" on the overview's card: the add sheet on today, once
		// (after the navigation: the router takes no `replaceState` before).
		if (page.url.searchParams.has('add')) {
			const url = new URL(page.url.href);
			url.searchParams.delete('add');
			replaceState(url, page.state);
			openAdd();
		}
		if (from?.route.id === '/plans/[id]' && from.params?.id === to?.params?.id) {
			// A link to this plan with other days (the load reads no query): there.
			if (type === 'link') follow(page.url.searchParams.get('from'));
			return;
		}
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
			(
				card?.querySelector<HTMLElement>('a[href], [role="link"], [data-entry-menu]') ?? target
			).focus({
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
		await goto(resolve('/plans'), { refreshAll: true, replaceState: true });
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
	const thisWeek = $derived(mondayOf(data.today));

	// Columns in view: a phone's week, from `md` the strip's (`DayStrip`).
	const seven = new MediaQuery('min-width: 1200px');
	const five = new MediaQuery('min-width: 1024px');
	const count = $derived(narrow.current || seven.current ? 7 : five.current ? 5 : 4);
	/** The strip's own place: today in the middle of the days in view. */
	const centered = $derived(aroundToday(data.today, count));
	const origin = () => (narrow.current ? data.today : centered);

	// What is in view. A phone shows the week of `rested`; from `md` the strip
	// shows `count` days from `start`, which follows its scroll frame by
	// frame (the heading's dates), while `rested` follows it once it rests:
	// the badge, the list stamps, the address and a screen reader take that,
	// so they do not flicker while the days pass. `around`: the address has
	// no `from`, the days sit around today (a phone: this week), and stay
	// there when the columns change.
	// svelte-ignore state_referenced_locally
	let around = $state(data.start === null);
	// svelte-ignore state_referenced_locally
	let start = $state(data.start ?? origin());
	// svelte-ignore state_referenced_locally
	let rested = $state(data.start ?? origin());
	/** The strip's first day in view on the timeline's ruler (`dayNumber`). */
	// svelte-ignore state_referenced_locally
	let at = $state(dayNumber(start));
	let view = $state<WeekView>();

	/** Shows the days from `from` (null: around today): a load's, a link's. */
	function follow(from: string | null) {
		around = from === null;
		const day = from ?? origin();
		if (day === rested) return;
		start = rested = day;
		view?.aim(day);
	}
	// A load from elsewhere (the phone's pager, a stamp's jump, Back) shows
	// its day; the strip's own loads ask for the days it already shows.
	// svelte-ignore state_referenced_locally
	let asked = data.start;
	$effect(() => {
		const next = data.start;
		untrack(() => {
			if (next === asked) return;
			asked = next;
			follow(next);
		});
	});
	// A resized window, a turned tablet, a new day: around today stays
	// around today, and the days now in view get loaded.
	$effect(() => {
		void centered;
		void narrow.current;
		untrack(() => {
			if (around) follow(null);
			sync();
		});
	});

	const monday = $derived(mondayOf(rested));
	const loaded = $derived(daysFrom(plan.from, plan.to));
	const last = (from: string) => shiftDay(from, count - 1);
	/** The days in view (the heading's) and at rest (the rest's). */
	const viewFrom = $derived(narrow.current ? monday : start);
	const restFrom = $derived(narrow.current ? monday : rested);
	const restTo = $derived(last(restFrom));
	const current = $derived(
		narrow.current ? monday === thisWeek : data.today >= viewFrom && data.today <= last(viewFrom)
	);
	const pill = (from: string) =>
		narrow.current ? weekFromNow(from, thisWeek) : windowFromNow(from, data.today, count);
	/** "Heute" rests where it leads: this week on a phone, today in the middle from `md`. */
	const atToday = $derived(narrow.current ? current : start === centered);
	/** The add sheet's days: the week from the first day at rest. */
	const days = $derived(weekDays(restFrom));
	/** The entries at rest in view, and the tray's. */
	const inView = $derived(shownEntries(plan.entries, restFrom, restTo));
	const unlisted = $derived(countUnlisted(inView, data.today));

	// A page turn by the pager slides the heading and the days in from the
	// side the week came from (a later week from the right), so the step
	// shows; reduced motion only fades. Only the pager's turn (`turning`):
	// not a refetch, nor the week moving on at midnight. The heading never
	// goes blank, and an empty week skips its note's own entrance.
	let heading = $state<HTMLElement>();
	let content = $state<HTMLElement>();
	let shown = '';
	let turning = false;
	$effect(() => {
		const before = shown;
		shown = monday;
		if (!before || before === monday || !turning) return;
		turning = false;
		const still = prefersReducedMotion.current;
		const from = `translateX(${monday > before ? '' : '-'}1.25rem)`;
		const frames = still
			? [{ opacity: 0.35 }, { opacity: 1 }]
			: [
					{ opacity: 0.35, transform: from },
					{ opacity: 1, transform: 'none' }
				];
		if (untrack(() => empty)) content?.getAnimations({ subtree: true }).forEach((a) => a.finish());
		untrack(() => [heading, content]).forEach((el) =>
			el?.animate(frames, { duration: still ? 160 : 260, easing: 'cubic-bezier(0.4, 0, 0.2, 1)' })
		);
	});
	// A phone's week is empty without a meal on its days or the tray; the
	// strip, without one in all its weeks (it scrolls past an empty week).
	const empty = $derived((narrow.current ? inView : plan.entries).length === 0);
	/** Every list the days at rest and the tray went to, as far as the reader can open them; most used first (`byPlanUse`). */
	const listStamps = $derived(
		byPlanUse(listsOf(plan.lists, inView)).map((l) => ({
			key: l.id,
			kind: 'list' as const,
			label: l.name,
			open: l.openCount,
			href: listHref(l.id)
		}))
	);
	const wide = new MediaQuery('min-width: 1024px');

	$effect(() => rememberPlan(plan.id));

	// The page's today is the real one: back on the tab on a new day, the load
	// runs again, so the cards' stamps, the folded days and the badge move on
	// together (the overview rereads the same way).
	// Focus and visibility arrive together on one tab switch: one load for both.
	let moved = '';
	function newDay() {
		const now = todayIn(getLocalTimeZone()).toString();
		if (now === data.today || now === moved) return;
		moved = now;
		void invalidateAll();
	}
	// A tab that stays in view past midnight (a kitchen tablet) moves on too.
	$effect(() => {
		let timer: ReturnType<typeof setTimeout>;
		const arm = () => {
			const now = new Date();
			const next = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1, 0, 0, 1);
			timer = setTimeout(() => {
				newDay();
				arm();
			}, next.getTime() - now.getTime());
		};
		arm();
		return () => clearTimeout(timer);
	});

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
	 * A phone's other week of this plan; this week's address carries no
	 * `from`. It replaces the shown week in the history, so Back leaves the
	 * plan. A tap while a week loads steps on from that week (`aim`) and
	 * abandons its navigation, whose settling must not end the latest turn's
	 * dim or slide.
	 */
	function show(from: string) {
		const mine = ++turn;
		aim = from;
		turning = pending = true;
		void goto(address(from === thisWeek ? null : from), {
			reset: false,
			replace: true,
			refreshAll: true
		}).finally(() => {
			if (mine === turn) turning = pending = false;
		});
	}
	/** The plan's address from `from`; null, around today. */
	const address = (from: string | null) => planHref(plan.id, from ?? undefined);

	/**
	 * The address follows the days at rest, and the loaded days cover them:
	 * a `goto` that replaces the plan's entry, running no load while they do
	 * (the load reads its query untracked), so a reload or Back opens there;
	 * one that loads (`refreshAll`) when they do not, from `md` within four
	 * days of either end of the six weeks. Never while a navigation is on its
	 * way (a card's recipe, the sidebar): it would cancel it, or replace the
	 * page it lands on. A write that was skipped or overtaken is made up after.
	 */
	let writing = false;
	function sync() {
		if (writing || navigating.to || page.route.id !== '/plans/[id]') return;
		const from = around ? null : rested;
		// From md a week of days beyond the view either way, so a week's step fits.
		const need = narrow.current
			? [monday, shiftDay(monday, 6)]
			: [shiftDay(rested, -7), shiftDay(rested, count + 6)];
		const load = !need.every((day) => loaded.includes(day));
		if (!load && from === page.url.searchParams.get('from')) return;
		writing = true;
		if (load) asked = from;
		goto(address(from), { reset: false, replace: true, refreshAll: load }).then(
			() => {
				writing = false;
				sync();
			},
			() => (writing = false)
		);
	}
	afterNavigate(() => sync());

	/** The strip at rest on `first`. */
	function settled(first: string) {
		rested = first;
		around = first === centered;
		sync();
	}
	/** The strip scrolls no further than its last `count` loaded days. */
	const reaches = (day: string) => {
		const at = loaded.indexOf(day);
		return at >= 0 && at <= loaded.length - count;
	};
	/**
	 * The pager from `md`: a day (a week with Shift) along the strip. With the
	 * strip not there (no meals in its weeks), the days move without it.
	 * On by `by` days from where the strip is going: along the strip when it
	 * holds them, else by a jump that loads them first, so a week step is a
	 * week even near the end of the loaded days.
	 */
	function stepBy(by: number) {
		if (!view || empty) return jumpTo(shiftDay(rested, by));
		const to = shiftDay(view.aimed() ?? rested, by);
		if (reaches(to)) view.scrollByDays(by);
		else jumpTo(to);
	}
	/** `day` first; loaded around it when it lies beyond the loaded weeks. */
	function jumpTo(day: string) {
		around = day === centered;
		if (view && !empty) view.aim(day);
		if (view && !empty && reaches(day)) return;
		start = rested = day;
		sync();
	}
	let turn = 0;
	let aim = '';
	/** The pager's next week is loading: the shown one dims and is busy. */
	let pending = $state(false);
	/** The week the pager steps from: the one on its way, else the shown one. */
	const step = (by: number) => show(shiftWeek(pending ? aim : monday, by));

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
	function openAdd(day: string | null = days.includes(data.today) ? data.today : restFrom) {
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

<svelte:head
	><title>{title} · {dayRange(restFrom, restTo, locale)} · {m.app_name()}</title></svelte:head
>

<svelte:window onfocus={newDay} onpageshow={newDay} />
<svelte:document onvisibilitychange={() => !document.hidden && newDay()} />

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
				planId={plan.id}
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
	     and "Put on the list". The pager keeps the right edge at every width,
	     so it stays under the finger whatever the dates' length; from md the
	     button sits before it and the lists take the full width below (one
	     line from lg, else two, then "+n"); on a phone all stacked, the
	     button over the full width. -->
	<div class="mb-4 flex flex-col gap-2.5 md:flex-row md:flex-wrap md:items-center md:gap-x-3">
		<div class="flex items-center justify-between gap-2 md:contents">
			<!-- Where the days in view lie sits over their dates, a pill for every
			     week or window: one that holds today wears today's accent. The
			     dates never run under the pager. A screen reader hears it once
			     the days rest (the live line after it), not while they pass. -->
			<h2
				bind:this={heading}
				class="flex min-w-0 flex-col items-start font-display text-card-sm font-medium whitespace-nowrap min-[360px]:text-card md:mr-auto md:text-heading"
			>
				<span
					data-week-from-now
					class="mb-1 -ml-0.5 inline-flex items-center gap-1.5 rounded-pill py-px font-sans text-caption max-[359.98px]:text-micro {current
						? 'bg-accent pr-2 pl-1.5 font-semibold text-accent-foreground'
						: 'bg-surface px-2 text-text ring-1 ring-border'}"
				>
					{#if current}<span class="size-1.5 rounded-full bg-primary" aria-hidden="true"
						></span>{/if}
					{pill(viewFrom)}</span
				>
				<span class="max-w-full truncate max-[1199.98px]:hidden"
					>{dayRange(viewFrom, last(viewFrom), locale)}</span
				>
				<span class="max-w-full truncate max-[359.98px]:hidden min-[1200px]:hidden"
					>{dayRange(viewFrom, last(viewFrom), locale, 'short')}</span
				>
				<span class="max-w-full truncate min-[360px]:hidden"
					>{dayRange(viewFrom, last(viewFrom), locale, 'numeric')}</span
				>
			</h2>
			<p class="sr-only" aria-live="polite">
				{pill(restFrom)}, {dayRange(restFrom, restTo, locale)}
			</p>
			<div class="flex shrink-0 items-center gap-1 md:order-2 md:gap-1.5">
				<IconButton
					label={narrow.current ? m.week_previous() : m.week_day_previous()}
					title={narrow.current
						? undefined
						: `${m.week_day_previous()} · ${m.week_day_step_hint()}`}
					class="max-md:size-11"
					onclick={(event) => (narrow.current ? step(-1) : stepBy(event.shiftKey ? -7 : -1))}
				>
					<ChevronLeft class="size-4.5" aria-hidden="true" />
				</IconButton>
				<!-- Where it would lead (this week on a phone, today first from md)
				     it rests, a label without its outline that stays a Tab stop, so
				     the focus stays put; elsewhere it is the way back. -->
				<Button
					variant="secondary"
					aria-disabled={atToday || undefined}
					label={atToday ? undefined : narrow.current ? m.week_today_jump() : m.week_today_first()}
					class="aria-disabled:cursor-default aria-disabled:border-transparent aria-disabled:text-text-muted aria-disabled:active:scale-100 max-md:h-11 max-md:px-3.5"
					onclick={() => !atToday && (narrow.current ? show(thisWeek) : jumpTo(centered))}
				>
					{m.week_today()}
				</Button>
				<IconButton
					label={narrow.current ? m.week_next() : m.week_day_next()}
					title={narrow.current ? undefined : `${m.week_day_next()} · ${m.week_day_step_hint()}`}
					class="max-md:size-11"
					onclick={(event) => (narrow.current ? step(1) : stepBy(event.shiftKey ? 7 : 1))}
				>
					<ChevronRight class="size-4.5" aria-hidden="true" />
				</IconButton>
			</div>
		</div>
		{#if listStamps.length > 0 || (!narrow.current && !empty)}
			<!-- A row of their own under the date from md: the bar keeps the
			     pager and the button, the lists get the width. From md the row
			     keeps its line when the days at rest went to no list, so the
			     timeline and the days do not jump as the strip passes them. -->
			<!-- Below sm the label sits above its row, which gets the full width. -->
			<div
				class="flex min-h-6 min-w-0 items-start gap-x-2 max-sm:flex-col max-sm:gap-y-0.5 md:order-last md:w-full"
			>
				{#if listStamps.length > 0}
					<span class="shrink-0 text-caption leading-6 text-text-muted" aria-hidden="true"
						>{m.plans_card_lists()}</span
					>
					<StampRow
						label={m.link_plan_lists()}
						lines={wide.current ? 1 : 2}
						stamps={listStamps}
						class="w-full flex-1"
					/>
				{/if}
			</div>
		{/if}
		{#if !empty}
			<div class="flex min-w-0 flex-col gap-1 md:order-1 md:flex-row md:items-center md:gap-3">
				<!-- The count is what the dialog would add for the shown week and the tray. -->
				<Button
					variant="primary"
					aria-haspopup="dialog"
					aria-expanded={listing}
					label={unlisted > 0
						? `${m.plan_to_list()}, ${unlisted === 1 ? m.plan_to_list_count_one() : m.plan_to_list_count({ count: unlisted })}`
						: undefined}
					onclick={() => {
						// The days the strip is on its way to, not the ones it left.
						if (!narrow.current && view && !empty) rested = view.aimed() ?? rested;
						listingOne = null;
						listing = true;
					}}
					class="shrink-0 shadow-cta max-md:h-11 max-md:w-full md:px-4.5"
				>
					<ShoppingBasket class="size-4" aria-hidden="true" />
					{m.plan_to_list()}
					{#if unlisted > 0}
						<span
							class="inline-flex h-5.5 min-w-5.5 items-center justify-center rounded-full bg-primary-foreground px-1.5 text-caption font-bold text-primary"
							aria-hidden="true">{unlisted}</span
						>
					{/if}
				</Button>
			</div>
		{/if}
	</div>

	{#if !narrow.current && !empty}
		<div class="-mt-1 mb-3">
			<DayTimeline
				days={loaded}
				today={data.today}
				entries={plan.entries}
				{at}
				{count}
				label={dayRange(start, last(start), locale)}
				onseek={(day) => view?.seekDay(day)}
				onrelease={() => view?.release()}
				onpick={jumpTo}
				onstep={stepBy}
			/>
		</div>
	{/if}

	<div
		bind:this={content}
		aria-busy={pending || undefined}
		class="transition-opacity {pending ? 'opacity-60' : ''}"
	>
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
				bind:this={view}
				{week}
				today={data.today}
				{monday}
				bind:start
				onat={(day) => (at = day)}
				{count}
				onsettle={settled}
				bind:expanded={() => expandedWeek === monday, (open) => (expandedWeek = open ? monday : '')}
				onadd={openAdd}
			/>
		{/if}
	</div>
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

<ToListDialog
	bind:open={listing}
	{week}
	range={{ from: restFrom, to: restTo }}
	weekly={narrow.current}
	today={data.today}
	only={listingOne}
/>
<AddEntrySheet bind:open={adding} {days} day={addDay} today={data.today} onadd={add} />
