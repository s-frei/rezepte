<script lang="ts" module>
	export type StampProps = {
		kind: 'plan' | 'list';
		label: string;
		/** A plan stamp's day, `YYYY-MM-DD`: "Mo", and "Mo 16.11." where its container is wide enough. */
		day?: string | null;
		/** A list stamp's open rows: "· 38 offen". */
		open?: number | null;
		/** Resolved; a link where the reader can open the other side. */
		href?: string | null;
		/** A plan the reader is not in: hairline, no tint, no link; its label names the kind ("Geplant von Mila"). */
		quiet?: boolean;
		/** Overrides the accessible name. */
		ariaLabel?: string;
		/** On a card a finger drags: a `CardLink`, no link preview on a long press. */
		card?: boolean;
	};

	/** A stamp's tint by kind; the row's summary pill ("3 Listen") wears it too. */
	export function stampTone(kind: StampProps['kind'], quiet = false): string {
		if (quiet) return 'text-text-muted ring-1 ring-inset ring-border';
		return kind === 'plan'
			? 'bg-plan-soft text-plan-foreground ring-1 ring-inset ring-plan-foreground/20'
			: 'bg-success-soft text-success-foreground';
	}
</script>

<script lang="ts">
	import CalendarDays from '@lucide/svelte/icons/calendar-days';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import { dayNames, stampDate } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import CardLink from '#lib/components/ui/CardLink.svelte';

	/**
	 * The link between a plan and a list, on either side: the kind's icon
	 * (calendar: a plan, basket: a list) and tint, its name, and a detail
	 * after a dot. The name gives way last: a plan's date drops to the
	 * weekday below a 15rem container (`@container` on an ancestor), then
	 * the name truncates. A link where the reader can open the other side,
	 * plain text where they cannot; the accessible name says the kind.
	 * On a phone a link's target grows to 44px without moving the line.
	 */
	let {
		kind,
		label,
		day = null,
		open = null,
		href = null,
		quiet = false,
		ariaLabel,
		card = false,
		wrap = false,
		bare = false,
		class: extra = ''
	}: StampProps & {
		/** Without its detail (the day, the open count), where the row is tight; the accessible name keeps it. */
		bare?: boolean;
		/** A popover's stamp: the name wraps, the detail runs on after it. */
		wrap?: boolean;
		class?: string;
	} = $props();

	const locale = getLocale();
	const names = $derived(day ? dayNames(day, locale) : null);
	const aria = $derived(
		ariaLabel ??
			[
				// "Geplant von Mila" and "Mein Plan" name the kind themselves.
				quiet || (kind === 'plan' && label === m.plan_personal())
					? label
					: m.link_aria({
							kind: kind === 'plan' ? m.link_kind_plan() : m.link_kind_list(),
							name: label
						}),
				names?.full,
				open === null ? null : m.list_open({ open })
			]
				.filter(Boolean)
				.join(', ')
	);
	const tone = $derived(stampTone(kind, quiet));
	const linked =
		'relative transition hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary max-md:after:absolute max-md:after:inset-x-0 max-md:after:inset-y-[calc(50%-22px)]';
	const base = $derived(
		`inline-flex h-6 max-w-full min-w-0 items-center gap-1 rounded-pill px-2 text-caption font-semibold ${wrap ? '' : 'whitespace-nowrap'} ${tone} ${extra}`
	);
</script>

{#snippet inner()}
	{#if kind === 'plan'}
		<CalendarDays class="size-3.5 shrink-0" aria-hidden="true" />
	{:else}
		<ShoppingBasket class="size-3.5 shrink-0" aria-hidden="true" />
	{/if}
	{#if wrap}
		<span class="min-w-0"
			>{label}{#if open !== null}<span class="font-normal whitespace-nowrap"
					>&nbsp;·&nbsp;{m.list_open({ open })}</span
				>{/if}</span
		>
	{:else}
		<span class="min-w-0 truncate">{label}</span>
	{/if}
	{#if names && day && !bare}
		<span class="shrink-0 font-normal"
			>·&nbsp;{names.weekday}<span class="hidden @[15rem]:inline">&nbsp;{stampDate(day)}</span
			></span
		>
	{/if}
	{#if open !== null && !wrap && !bare}
		<span class="shrink-0 font-normal">·&nbsp;{m.list_open({ open })}</span>
	{/if}
{/snippet}

{#if href && card}
	<CardLink {href} aria-label={aria} data-stamp class="{base} {linked}">{@render inner()}</CardLink>
{:else if href}
	<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- resolved by the caller -->
	<a {href} aria-label={aria} draggable="false" data-stamp class="{base} {linked}"
		>{@render inner()}</a
	>
{:else}
	<span class={base}
		><span class="sr-only">{aria}</span><span aria-hidden="true" class="contents"
			>{@render inner()}</span
		></span
	>
{/if}
