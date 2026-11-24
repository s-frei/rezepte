<script lang="ts">
	import { on } from 'svelte/events';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import StampRow from '#lib/components/shopping/StampRow.svelte';
	import { getToList } from '#lib/mealplan/to-list.js';
	import { listEntryHref } from '#lib/shopping/href.js';
	import { byListName } from '#lib/shopping/plan-ref.js';
	import { imageUrl } from '#lib/api/recipes.js';
	import { objectFocus } from '#lib/images/focus.js';
	import type { PlanEntry } from '#lib/api/mealplan.js';
	import CardLink from '#lib/components/ui/CardLink.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { recipeHref } from '#lib/mealplan/entries.js';
	import { m } from '#lib/paraglide/messages.js';
	import { initialFor, tintFor } from '#lib/recipe/placeholder.js';
	import EntryMenu from './EntryMenu.svelte';

	/**
	 * One planned meal. `column` is the desktop's small recipe card, photo on
	 * top; `row` the compact line of a day row or a phone's day card, the
	 * thumbnail beside the title; `chip` the tray strip's pill. A free text
	 * is a dashed card in every layout, its words in italic Literata.
	 */
	let {
		entry,
		layout,
		today,
		onmove,
		onservings,
		onremove
	}: {
		entry: PlanEntry;
		layout: 'column' | 'row' | 'chip';
		/** `YYYY-MM-DD`, the page's today: a day before it is past. */
		today: string;
		onmove: () => void;
		onservings: (servings: number) => void;
		onremove: () => void;
	} = $props();

	const href = $derived(recipeHref(entry));
	const toList = getToList();
	/** A recipe on no list, today or later or on the tray, says so and offers one. */
	const unlisted = $derived(
		entry.recipeId !== null &&
			entry.servings !== null &&
			entry.lists.length === 0 &&
			(entry.day === null || entry.day >= today)
	);
	const servings = $derived(
		entry.servings === null
			? ''
			: entry.servings === 1
				? m.plan_servings_one()
				: m.plan_servings({ count: entry.servings })
	);
	const by = $derived(entry.createdBy ? m.plan_by({ name: entry.createdBy.displayName }) : '');

	// A link's own long-press menu and a word's selection would take over
	// from the drag a held finger starts; the card has its ⋯ instead.
	const still =
		'select-none [-webkit-touch-callout:none] [-webkit-tap-highlight-color:transparent]';
	// The whole card opens the recipe: the title's link stretched over it.
	const stretched =
		'outline-none after:absolute after:inset-0 after:rounded-[inherit] focus-visible:after:outline-2 focus-visible:after:outline-offset-2 focus-visible:after:outline-primary';
	// The tap the zone repeats after a touch that never became a drag
	// (svelte-dnd-action's own click) would open the page a second time.
	const once = (event: MouseEvent) => {
		if (!event.isTrusted) event.preventDefault();
	};

	// A tray chip's drag face exists only while a press could become a drag:
	// rendered on the press, before the library copies the card at the
	// drag's start, and gone when it ends, so the page holds every title once.
	let armed = $state(false);
	function arm() {
		armed = true;
		const disarm = () => {
			armed = false;
			stops.forEach((stop) => stop());
		};
		const stops = [
			on(window, 'pointerup', disarm, { capture: true }),
			on(window, 'pointercancel', disarm, { capture: true })
		];
	}
</script>

{#snippet title(cls: string)}
	{#if href}
		<CardLink {href} onclick={once} class="{cls} {stretched}">{entry.title}</CardLink>
	{:else}
		<span class={cls}>{entry.title}</span>
	{/if}
{/snippet}

{#snippet cover(cls: string, letter: string)}
	{#if entry.recipeId && entry.photo}
		<img
			src={imageUrl(entry.recipeId, entry.photo, 'thumb')}
			alt=""
			{@attach objectFocus(() => ({
				focus: entry.photoFocus,
				width: entry.photoWidth,
				height: entry.photoHeight
			}))}
			loading="lazy"
			draggable="false"
			class="block object-cover [-webkit-user-drag:none] {cls}"
		/>
	{:else}
		<span
			aria-hidden="true"
			class="flex items-center justify-center font-display font-medium text-primary italic {tintFor(
				entry.recipeId ?? entry.id
			)} {letter} {cls}">{initialFor(entry.title)}</span
		>
	{/if}
{/snippet}

{#snippet mark(cls: string)}
	{#if entry.createdBy}
		<span title={by} class="flex shrink-0 {cls}">
			<PersonMark person={entry.createdBy} size="xs" />
			<span class="sr-only">{by}</span>
		</span>
	{/if}
{/snippet}

{#snippet onList()}
	{#if entry.lists.length > 0}
		<!-- Each list a link, two lines at most, then "+n"; above the stretched title link. -->
		<StampRow
			class="relative z-10 w-full"
			label={m.plans_card_lists()}
			stamps={byListName(entry.lists).map((l) => ({
				key: l.id,
				kind: 'list' as const,
				label: l.name,
				href: listEntryHref(l.id, entry.id),
				card: true
			}))}
		/>
	{:else if unlisted && toList}
		<!-- A press here is its own, never the start of a drag of the card. -->
		<button
			type="button"
			aria-haspopup="dialog"
			aria-label={m.link_not_listed_label({ title: entry.title })}
			onmousedown={(event) => event.stopPropagation()}
			ontouchstart={(event) => event.stopPropagation()}
			onclick={(event) => {
				// The card's drag swallows the press, so the click alone would leave
				// the focus on the page; the dialog gives it back to what held it.
				event.currentTarget.focus();
				toList(entry);
			}}
			class="relative z-10 inline-flex h-6 max-w-full min-w-0 items-center gap-1 rounded-pill border border-dashed border-handle px-2 text-caption font-semibold whitespace-nowrap text-text-muted transition hover:border-primary hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary max-md:after:absolute max-md:after:inset-x-0 max-md:after:inset-y-[calc(50%-22px)] dark:border-text-muted/60"
		>
			<ShoppingBasket class="size-3.5 shrink-0" aria-hidden="true" />
			<span class="truncate">{m.link_not_listed()}</span>
		</button>
	{/if}
{/snippet}

{#snippet menu(trigger: string)}
	<EntryMenu {entry} {trigger} {onmove} {onservings} {onremove} />
{/snippet}

{#snippet face(l: 'column' | 'row' | 'chip')}
	{#if entry.recipeId === null && entry.servings === null}
		<!-- A note: dashed, italic, the adder's mark beside it. -->
		<div
			class="group relative flex min-h-11 gap-2 rounded-lg border-[1.5px] border-dashed border-handle {still} {l ===
			'column'
				? 'items-start py-2.5 pr-1.5 pl-3'
				: 'items-center pl-3.5 md:min-h-10 md:rounded-2xl md:pr-[2.5px]'}"
		>
			<span
				class="min-w-0 font-display break-words [hyphens:none] text-text italic {l === 'column'
					? 'flex-1 text-body-sm leading-snug'
					: 'text-body leading-snug'}">{entry.title}</span
			>
			{#if l === 'column'}
				<!-- The mark over the ⋯, so a column's narrow card leaves the words their
				     width. On a pointer that hovers, the ⋯ waits for the card; focus shows it too. -->
				<span class="-my-1 flex shrink-0 flex-col items-center gap-1">
					{@render mark('mt-1')}
					{@render menu(
						'size-7 text-text-muted hover:bg-background [@media(hover:hover)]:opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 focus-visible:opacity-100 aria-expanded:opacity-100'
					)}
				</span>
			{:else}
				{@render mark('mr-auto')}
				<!-- From md 40px tall, the ⋯ (radius 16) 4px in from the outer edge, the
				     1.5px border included: concentric with the 20. -->
				{@render menu('size-11 text-text-muted md:size-8 hover:bg-background')}
			{/if}
		</div>
	{:else if l === 'column'}
		<article class="relative overflow-hidden rounded-xl bg-surface shadow-card {still}">
			<div class="relative">
				{@render cover('aspect-4/3 w-full', 'text-display-sm')}
				{@render mark('absolute -bottom-2 left-2 rounded-full ring-2 ring-surface')}
			</div>
			<div class="flex flex-col gap-0.5 px-2.5 pt-3 pb-2.5">
				{@render title('text-body-sm leading-snug font-semibold text-text [hyphens:none]')}
				<span class="text-caption whitespace-nowrap text-text-muted">{servings}</span>
				<div class="mt-1 flex min-w-0">{@render onList()}</div>
			</div>
			<!-- 4px in, so the ⋯ (radius 14) is concentric with the card's 18. -->
			<div class="absolute top-1 right-1 z-10">
				{@render menu('size-7 bg-surface text-text shadow-card hover:bg-background')}
			</div>
		</article>
	{:else}
		<!-- The thumbnail beside the title and servings, the ⋯ beside them at
		     the top; the lists below run on under the ⋯, the row's full width. -->
		<article
			class="relative grid min-h-13 grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-x-3 rounded-lg {still} {l ===
			'chip'
				? 'md:min-h-0 md:rounded-xl md:bg-background md:p-1'
				: ''}"
		>
			<span class="relative row-span-2 shrink-0">
				{@render cover(
					l === 'chip' ? 'size-11 rounded-md md:rounded-lg' : 'size-12 rounded-md',
					'text-heading'
				)}
				{@render mark('absolute -right-1 -bottom-1 rounded-full ring-2 ring-surface')}
			</span>
			<div class="col-start-2 row-start-1 flex min-w-0 self-end">
				{@render title(`text-body-sm leading-snug font-semibold text-text [hyphens:none]`)}
			</div>
			<span
				class="col-start-2 row-start-2 self-start text-caption whitespace-nowrap text-text-muted"
				>{servings}</span
			>
			<!-- On a tray chip from md, the ⋯ and the thumbnail sit 4px in: radius
			     14 each, concentric with the chip's 18. -->
			<span
				class="relative z-10 col-start-3 row-span-2 row-start-1 flex {l === 'chip'
					? 'md:self-start'
					: ''}"
			>
				{@render menu(
					`text-text-muted hover:bg-surface ${l === 'chip' ? 'size-11 md:size-7' : 'size-11 md:size-9'}`
				)}
			</span>
			{#if entry.lists.length > 0 || (unlisted && toList)}
				<!-- Below 360 the lists start under the thumbnail: two short ones share a line. -->
				<div
					class="col-span-2 col-start-2 row-start-3 mt-1 flex min-w-0 max-[359px]:col-span-3 max-[359px]:col-start-1"
				>
					{@render onList()}
				</div>
			{/if}
		</article>
	{/if}
{/snippet}

{#if layout === 'chip'}
	<!-- Lifted out of the tray, the copy that follows the pointer is the card
	     the days show (the zone sizes it to a column), so a drag looks like
	     what lands in the week; on the page the face stays hidden and inert. -->
	<div {@attach (node) => on(node, 'pointerdown', arm)} class="in-[#dnd-action-dragged-el]:hidden">
		{@render face('chip')}
	</div>
	{#if armed}
		<div
			data-drag-face
			aria-hidden="true"
			inert
			class="hidden *:shadow-none in-[#dnd-action-dragged-el]:block dark:*:bg-surface-elevated"
		>
			{@render face('column')}
		</div>
	{/if}
{:else}
	{@render face(layout)}
{/if}
