<script lang="ts">
	import NotebookPen from '@lucide/svelte/icons/notebook-pen';
	import Plus from '@lucide/svelte/icons/plus';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import type { WeekEntry, WeekPlan } from '#lib/api/mealplan.js';
	import { imageUrl } from '#lib/api/recipes.js';
	import { session } from '#lib/auth.svelte.js';
	import PeopleStack from '#lib/components/shopping/PeopleStack.svelte';
	import StampRow from '#lib/components/shopping/StampRow.svelte';
	import { planHref } from '#lib/mealplan/href.js';
	import { cellKind, weekStrip } from '#lib/mealplan/overview.svelte.js';
	import { dayNames, mondayOf } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { initialFor, tintFor } from '#lib/recipe/placeholder.js';
	import { listHref } from '#lib/shopping/href.js';

	/**
	 * One plan on the overview: its name and people, this week as a strip of
	 * seven days, what is on today (a link to the plan on today), and a stamp
	 * per list the week feeds. The whole card opens the plan; the today line
	 * and the stamps are links of their own above it.
	 *
	 * Its four parts (name and meta, strip, today, lists) are rows of the
	 * overview's grid (`grid-rows-subgrid`), so they line up across a row of
	 * cards; each keeps one height (one line of meta, of today, of stamps)
	 * and every card has all four, so no card has a blank part.
	 */
	let { plan, today }: { plan: WeekPlan; today: string } = $props();

	const locale = getLocale();
	const meId = $derived(session.user?.id);
	const mine = $derived(plan.owner.id === meId);
	const name = $derived(plan.personal ? m.plan_personal() : (plan.name ?? ''));
	const people = $derived([plan.owner, ...plan.members]);
	const names = (list: typeof plan.members) =>
		list.map((p) => (p.id === meId ? m.lists_you() : p.displayName)).join(', ');
	// Short, so it fits one line on a phone: no faces already says "only
	// you", and the strip says "this week". Waiting invitees are named.
	const who = $derived(
		plan.personal
			? ''
			: !mine
				? m.plans_card_from({ owner: plan.owner.displayName })
				: plan.members.length > 0
					? m.plans_card_with({ names: names(plan.members) })
					: plan.pending.length > 0
						? m.plans_card_invited({ names: names(plan.pending) })
						: ''
	);
	const meals = $derived(plan.entries.length);
	// State first, then who shares it, as a list card leads with its counts.
	const meta = $derived(
		[
			meals > 1
				? m.plans_card_meals({ count: meals })
				: meals === 1
					? m.plans_card_meals_one()
					: plan.tray > 0
						? m.plans_card_tray({ count: plan.tray })
						: m.plans_card_week_none(),
			who
		]
			.filter(Boolean)
			.join(' · ')
	);
	const strip = $derived(weekStrip(plan.entries, mondayOf(today), today));
	const now = $derived(plan.entries.filter((e) => e.day === today));
	const href = $derived(planHref(plan.id));
	const titleId = $derived(`plan-card-${plan.id}`);
	const stamps = $derived(
		plan.lists.map((list) => ({
			key: list.id,
			kind: 'list' as const,
			label: list.name,
			open: list.openCount,
			href: listHref(list.id)
		}))
	);
	const sr = (day: string, when: string, meals: WeekEntry[]) => {
		const args = {
			day: dayNames(day, locale).full,
			meals: meals.length > 0 ? meals.map((e) => e.title).join(', ') : m.plan_past_days_none()
		};
		return when === 'today'
			? m.plans_day_sr_today(args)
			: when === 'past'
				? m.plans_day_sr_past(args)
				: m.plans_day_sr(args);
	};
</script>

{#snippet thumb(entry: WeekEntry)}
	{@const kind = cellKind(entry)}
	{#if kind === 'photo'}
		<img
			src={imageUrl(entry.recipeId ?? '', entry.photo ?? '', 'thumb')}
			alt=""
			loading="lazy"
			class="size-full object-cover"
		/>
	{:else if kind === 'letter'}
		<!-- A recipe without a photo, or one deleted since: its letter, as on the week. -->
		<span
			class="flex size-full items-center justify-center font-display text-body-sm font-medium text-primary italic {tintFor(
				entry.recipeId ?? entry.id
			)}">{initialFor(entry.title)}</span
		>
	{:else}
		<!-- A note: a pencil on a solid frame (planned), never a letter. -->
		<span
			class="flex size-full items-center justify-center rounded-md border border-border bg-surface-elevated text-text-muted"
		>
			<NotebookPen class="size-3.5" />
		</span>
	{/if}
{/snippet}

<article
	aria-labelledby={titleId}
	class="group relative row-span-4 grid grid-rows-subgrid gap-0 rounded-2xl bg-surface px-4 py-4.5 shadow-card transition-[translate,scale,box-shadow,background-color] duration-160 ease-out focus-within:shadow-lift hover:shadow-lift hover:delay-60 hover:duration-420 hover:ease-spring active:scale-[.985] motion-safe:hover:-translate-y-1 motion-reduce:transition-none min-[360px]:px-5 dark:hover:bg-surface-elevated"
>
	<!-- At the row's foot: when a neighbor's name wraps, the room goes above
	     this title, and the meta stays on the strip it describes. -->
	<div class="min-w-0 self-end">
		<div class="flex items-start justify-between gap-3">
			<h3
				id={titleId}
				class="min-w-0 font-display text-card font-medium wrap-break-word hyphens-auto"
			>
				<!-- Stretched over the card: the whole card opens the plan. -->
				<a
					{href}
					class="transition-colors outline-none group-hover:text-primary after:absolute after:inset-0 after:rounded-2xl focus-visible:after:outline-2 focus-visible:after:outline-offset-2 focus-visible:after:outline-primary"
					>{name}</a
				>
			</h3>
			{#if people.length > 1}<PeopleStack {people} />{/if}
		</div>
		<!-- One line, so cards side by side keep one height; the faces say who. -->
		<p class="mt-0.5 truncate text-caption text-text-muted">{meta}</p>
	</div>

	<!-- This week at a glance: one cell per day. -->
	<ol class="mt-3.5 grid grid-cols-7 gap-2 self-start md:gap-1.5" aria-label={m.week_this()}>
		{#each strip as cell (cell.day)}
			{@const dn = dayNames(cell.day, locale)}
			{@const isToday = cell.when === 'today'}
			{@const past = cell.when === 'past'}
			<li
				aria-current={isToday ? 'date' : undefined}
				class="flex min-w-0 flex-col items-center gap-1.5"
			>
				<span
					aria-hidden="true"
					class="text-micro {isToday ? 'font-bold text-primary' : 'font-semibold text-text-muted'}"
					>{dn.weekday}</span
				>
				<span aria-hidden="true" class="relative block aspect-square w-full max-w-12">
					<!-- Dashed: a day still to plan. A free day gone by is just a dot. -->
					<span
						class="flex size-full items-center justify-center overflow-hidden rounded-md {cell.meals
							.length > 0
							? past
								? 'opacity-50'
								: ''
							: isToday || past
								? ''
								: 'border-[1.5px] border-dashed border-handle'} {isToday
							? 'ring-2 ring-primary ring-offset-1 ring-offset-surface md:ring-offset-2'
							: ''}"
					>
						{#if cell.meals.length > 0}
							{@render thumb(cell.meals[0])}
						{:else if past}
							<span class="size-1.5 rounded-full bg-handle"></span>
						{/if}
					</span>
					{#if cell.meals.length > 1}
						<!-- More meals that day: a sticker cut out of the cell's corner. -->
						<span
							class="absolute -right-1 -bottom-1 rounded-pill bg-surface px-1 text-label font-bold text-text ring-2 ring-surface {past
								? 'opacity-50'
								: ''}">+{cell.meals.length - 1}</span
						>
					{/if}
				</span>
				<span class="sr-only">{sr(cell.day, cell.when, cell.meals)}</span>
			</li>
		{/each}
	</ol>

	<!-- Today, a tap away: the plan opens on today; an empty today offers to plan it. -->
	{#if now.length > 0}
		<a
			href="{href}#today"
			aria-label={now.length > 1
				? m.plans_card_today_label_more({ title: now[0].title, count: now.length - 1 })
				: m.plans_card_today_label({ title: now[0].title })}
			class="relative z-10 mt-3 flex max-w-full min-w-0 items-baseline gap-2 self-start justify-self-start rounded-md text-body-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary max-md:after:absolute max-md:after:inset-x-0 max-md:after:-inset-y-3 [&:hover_.title]:underline"
		>
			<span class="shrink-0 font-semibold text-primary">{m.week_today()}</span>
			<span class="title min-w-0 truncate font-semibold text-text">{now[0].title}</span>
			{#if now.length > 1}
				<span class="-ml-1 shrink-0 text-caption whitespace-nowrap text-text-muted"
					>{m.plans_card_today_more({ count: now.length - 1 })}</span
				>
			{/if}
		</a>
	{:else}
		<a
			href="{href}?add#today"
			class="relative z-10 mt-3 flex items-center gap-1.5 self-start justify-self-start rounded-md text-body-sm font-medium text-text-muted hover:text-primary hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary max-md:after:absolute max-md:after:inset-x-0 max-md:after:-inset-y-3"
		>
			<Plus class="size-4 shrink-0" aria-hidden="true" />
			{m.plans_card_today_add()}
		</a>
	{/if}

	<!-- The lists this week went to; the row passes taps between its stamps to the card. -->
	{#if stamps.length > 0}
		<StampRow
			{stamps}
			lines={1}
			label={m.plans_card_lists()}
			class="pointer-events-none relative z-10 mt-3.5 self-start border-t border-border pt-3.5 max-md:gap-y-2.5"
			stampClass="pointer-events-auto"
		/>
	{:else if plan.unlisted > 0}
		<p
			class="mt-3.5 flex items-center gap-1.5 self-start border-t border-border pt-3.5 text-caption leading-6 text-text-muted"
		>
			<ShoppingBasket class="size-3.5 shrink-0" aria-hidden="true" />
			{m.plans_card_lists_none({ count: plan.unlisted })}
		</p>
	{:else}
		<p
			class="mt-3.5 flex items-center gap-1.5 self-start border-t border-border pt-3.5 text-caption leading-6 text-text-muted"
		>
			<ShoppingBasket class="size-3.5 shrink-0" aria-hidden="true" />
			{m.plans_card_lists_nothing()}
		</p>
	{/if}
</article>
