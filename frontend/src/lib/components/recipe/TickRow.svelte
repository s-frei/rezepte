<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import List from '@lucide/svelte/icons/list';
	import type { PantryItem } from '#lib/api/recipes.js';
	import { m } from '#lib/paraglide/messages.js';
	import { beyondCap, coveredBy, ownTick, ownTickCount, withTicks } from '#lib/recipe/ticks.js';
	import Button from '#lib/components/ui/Button.svelte';
	import ScrollFade from '#lib/components/ui/ScrollFade.svelte';
	import TickMark from '#lib/components/ui/TickMark.svelte';

	let {
		row,
		ticked,
		focus,
		ontoggle,
		onclear,
		ondone,
		hint,
		onhover
	}: {
		/** What the ranked cards miss, in order, once per key; frozen by the page. */
		row: PantryItem[];
		/** The user's own ticks. */
		ticked: PantryItem[];
		/** The card the row is focused on: only its missing items. */
		focus: { title: string; missing: PantryItem[] } | null;
		/** Ticks an item, or unticks the tick behind it (its own, or the one covering it). */
		ontoggle: (item: PantryItem) => void;
		/** "Clear ticks": the page clears them and offers the undo toast. */
		onclear: () => void;
		/** "Done" leaves the focused mode. */
		ondone: () => void;
		/** The first-use hint, until the first tick or card focus. */
		hint: boolean;
		/** A mouse over a pill names its ingredient (null on leaving), for the cards to mark. */
		onhover?: (item: PantryItem | null) => void;
	} = $props();

	/** Pills a desktop row shows before "+N more"; ticked ones always show. */
	const CAP = 11;

	const id = $props.id();
	// "+N more" opens this row only: a new row (any filter changed) starts
	// capped. Raw, since a proxied copy of `row` would never equal `row`.
	let expandedFor = $state.raw<PantryItem[] | null>(null);
	const expanded = $derived(expandedFor === row);
	let capped = $state(false);
	let atEnd = $state(false);

	const items = $derived(focus ? focus.missing : withTicks(row, ticked));
	// What the cap leaves out, opened or not: the same button opens the row
	// and folds it back.
	const overflow = $derived(focus ? [] : beyondCap(items, ticked, CAP));
	const hidden = $derived(new Set(expanded ? [] : overflow.map((i) => i.key)));
	const count = $derived(ownTickCount(ticked));
	const hintText = $derived(m.ticks_hint({ stamp: '\u0000' }).split('\u0000'));
	const focusText = $derived(
		!focus
			? []
			: (focus.missing.length > 0 ? m.ticks_focus : m.ticks_focus_all_there)({
					title: '\u0000'
				}).split('\u0000')
	);

	// The first tick takes the hint's line away; the row keeps the height it
	// had with the hint, or the sticky row and the grid under it would jump.
	// Only the folded row's height counts: one taken while "+N more" stood
	// open stayed after "Less" as an empty band. A new row starts over.
	let height = $state(0);
	let hinted = $state.raw<{ row: PantryItem[]; height: number } | null>(null);
	$effect(() => {
		if (hint && !focus && !expanded && height > 0) hinted = { row, height };
	});
	const keep = $derived(!hint && !focus && hinted?.row === row ? hinted.height : 0);

	// Up to five rows stand open, fewer on a short screen, which must keep
	// the focused card's lines in view under the sticky row; beyond that the
	// focused row scrolls inside, the last row peeking under a fade until
	// the end is reached.
	function measure(el: HTMLElement) {
		const update = () => {
			capped = el.scrollHeight > el.clientHeight + 1;
			atEnd = el.scrollTop + el.clientHeight >= el.scrollHeight - 1;
		};
		update();
		const observer = new ResizeObserver(update);
		for (const child of [el, ...el.children]) observer.observe(child);
		el.addEventListener('scroll', update, { passive: true });
		return () => {
			observer.disconnect();
			el.removeEventListener('scroll', update);
		};
	}

	const link =
		'relative inline-flex h-10 shrink-0 items-center rounded-pill px-1.5 whitespace-nowrap underline underline-offset-3 after:absolute after:-inset-x-1 after:-top-1 after:bottom-0 hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
	// "Clear ticks" on the phone's hint line: a finger's height of its own,
	// which the line keeps for the hint too, so its hit area covers neither
	// the heading above nor the pills below.
	const line =
		'relative inline-flex h-11 items-center rounded-pill whitespace-nowrap underline underline-offset-3 after:absolute after:-inset-x-1 after:inset-y-0 hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
</script>

{#snippet pill(item: PantryItem)}
	{@const by = coveredBy(item, ticked)}
	{@const on = ownTick(item, ticked) !== null}
	<button
		type="button"
		aria-pressed={on || by !== null}
		aria-label={by ? m.ticks_covered({ name: item.name, by: by.name }) : undefined}
		title={by ? m.ticks_covered({ name: item.name, by: by.name }) : undefined}
		data-key={item.key}
		onclick={() => ontoggle(by ?? item)}
		onpointerenter={(event) => event.pointerType === 'mouse' && onhover?.(item)}
		onpointerleave={(event) => event.pointerType === 'mouse' && onhover?.(null)}
		class="relative inline-flex h-10 shrink-0 items-center gap-1.75 rounded-pill border pr-3 pl-1.5 text-body-sm font-medium whitespace-nowrap transition-[background-color,border-color,color] after:absolute after:-inset-x-1 after:-inset-y-0.75 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {by
			? 'border-dashed border-handle bg-transparent text-text'
			: on
				? 'border-accent bg-accent text-accent-foreground'
				: 'border-border bg-surface text-text hover:border-handle'} {hidden.has(item.key)
			? 'md:hidden'
			: ''}"
	>
		<TickMark checked={on} covered={by !== null} size="md" />
		<span
			>{item.name}{#if by}<span class="text-caption text-text-muted">&nbsp;·&nbsp;{by.name}</span
				>{/if}</span
		>
	</button>
{/snippet}

{#snippet status(phone: boolean)}
	{#if count > 0}
		<span class="text-text-muted">{m.ticks_count({ count })}&nbsp;·</span>
		<button
			type="button"
			onclick={onclear}
			class="{phone ? line : link} font-semibold text-text-muted"
		>
			{m.ticks_clear()}
		</button>
	{:else if hint}
		<span class="text-text-muted">
			{hintText[0]}<span
				class="inline-flex items-center gap-1 rounded-pill bg-surface-elevated px-2 py-0.5 align-middle text-micro font-bold text-text shadow-card"
				><List class="size-3.5" aria-hidden="true" />{m.ticks_hint_stamp()}</span
			>{hintText[1]}
		</span>
	{/if}
{/snippet}

<!--
	The row of ticks under the search. On a phone a header
	line over one sideways line of pills, all of them; on a desktop the label
	left of wrapping pills, capped at CAP plus every ticked one until
	"+N more". Focused on a card it lists only that card's missing items and
	"Done", which sits in the header on a phone and at the right on a desktop:
	a two-column grid below `md`, a flex row from there with Done last.
-->
<div
	bind:offsetHeight={height}
	style:min-height={keep ? `${keep}px` : undefined}
	class="grid grid-cols-[minmax(0,1fr)_auto] content-start items-center md:flex md:items-start {focus
		? 'gap-x-3'
		: 'md:gap-x-3'}"
>
	<div
		class="flex min-h-10 min-w-0 flex-wrap items-center gap-x-1.5 text-body-sm md:flex-nowrap {focus
			? 'md:max-w-56 lg:max-w-none lg:shrink-0'
			: 'md:shrink-0'}"
	>
		{#if focus}
			<span id="{id}-label" class="font-bold">
				{focusText[0]}<em class="font-display text-body font-semibold text-primary italic"
					>{focus.title}</em
				>{focusText[1]}
			</span>
		{:else}
			<strong id="{id}-label">{m.ticks_heading()}</strong>
			<!-- The count takes the hint's line, both at a finger's height, so
			     neither the heading nor the pills move on the first tick. -->
			{#if count > 0 || hint}<span
					class="flex min-h-11 basis-full items-center gap-x-1.5 text-caption md:hidden"
					>{@render status(true)}</span
				>{/if}
		{/if}
	</div>

	{#if focus}
		{#if items.length > 0}<div
				role="group"
				aria-labelledby="{id}-label"
				{@attach measure}
				class="col-span-2 -m-1 mt-1 flex max-h-[min(--spacing(51),25dvh)] min-w-0 flex-wrap gap-1.5 p-1 md:-mt-1 md:flex-1 {capped
					? 'overflow-y-auto overscroll-contain'
					: ''} {capped && !atEnd ? 'mask-b-from-[calc(100%-18px)]' : ''}"
			>
				{#each items as item (item.key)}{@render pill(item)}{/each}
			</div>{/if}
		<!-- After the pills in the document, so Tab reaches it last; on a
		     phone it is placed back into the header's second column.
		     44px on a phone and under any finger, a dense row's height;
		     40px beside the pills for a mouse. -->
		<Button
			onclick={ondone}
			class="gap-1.5 font-bold max-md:col-start-2 max-md:row-start-1 max-md:h-11 md:shadow-cta pointer-coarse:h-11"
		>
			<Check class="size-4" strokeWidth={3} aria-hidden="true" />{m.ticks_done()}
		</Button>
	{:else}
		<!-- No negative top margin on a phone: the header's "Clear ticks"
		     keeps its whole hit area above the pills' own. The group is the
		     scroller itself, never a `display: contents` box, whose role
		     older Safari drops. -->
		<ScrollFade
			frame="col-span-2 -mx-1 -mb-1 min-w-0 md:-mt-1 md:flex-1"
			fade="from-background"
			arrows={{ previous: m.ticks_scroll_previous(), next: m.ticks_scroll_next() }}
			role="group"
			aria-labelledby="{id}-label"
			class="flex [scrollbar-width:none] gap-1.5 overflow-x-auto p-1 md:flex-wrap md:overflow-visible"
		>
			{#each items as item (item.key)}{@render pill(item)}{/each}
			{#if overflow.length > 0}
				<!-- One button both ways, so a keyboard keeps its focus; the
				     chevron turns with it, as on the tag row's, and says it
				     unfolds rather than reading on into the hint beside it. -->
				<button
					type="button"
					aria-expanded={expanded}
					onclick={() => (expandedFor = expanded ? null : row)}
					class="{link} gap-1 text-body-sm font-semibold text-text max-md:hidden"
				>
					{expanded ? m.ticks_less() : m.ticks_more({ count: overflow.length })}
					<ChevronDown
						class="size-4 shrink-0 transition-transform motion-reduce:transition-none {expanded
							? 'rotate-180'
							: ''}"
						aria-hidden="true"
					/>
				</button>
			{/if}
			<span class="hidden min-h-10 items-center gap-x-1.5 pl-1 text-body-sm md:flex">
				{@render status(false)}
			</span>
		</ScrollFade>
	{/if}
</div>
