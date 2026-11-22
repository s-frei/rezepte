<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
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
	/** Five rows of 40px pills with their 6px gaps and the scroller's padding. */
	const FIVE_ROWS = 5 * 40 + 4 * 6 + 8;

	const id = $props.id();
	// "+N more" opens this row only: a new row (new chips, text or tags) starts
	// capped. Raw, since a proxied copy of `row` would never equal `row`.
	let expandedFor = $state.raw<PantryItem[] | null>(null);
	const expanded = $derived(expandedFor === row);
	let capped = $state(false);
	let atEnd = $state(false);

	const items = $derived(focus ? focus.missing : withTicks(row, ticked));
	const hidden = $derived(
		new Set(focus || expanded ? [] : beyondCap(items, ticked, CAP).map((i) => i.key))
	);
	const count = $derived(ownTickCount(ticked));
	const hintText = $derived(m.ticks_hint({ stamp: '\u0000' }).split('\u0000'));
	const focusText = $derived(
		!focus
			? []
			: (focus.missing.length > 0 ? m.ticks_focus : m.ticks_focus_all_there)({
					title: '\u0000'
				}).split('\u0000')
	);

	// Up to five rows stand open; beyond that the focused row scrolls inside,
	// the fifth row peeking under a fade until the end is reached.
	function measure(el: HTMLElement) {
		const update = () => {
			capped = el.scrollHeight > FIVE_ROWS;
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
		'relative inline-flex h-10 shrink-0 items-center rounded-pill px-1.5 whitespace-nowrap underline underline-offset-3 after:absolute after:-inset-x-1 after:-inset-y-0.5 hover:text-primary focus-visible:outline-2 focus-visible:outline-primary';
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
		class="relative inline-flex h-10 shrink-0 items-center gap-1.75 rounded-pill border pr-3 pl-1.5 text-body-sm font-medium whitespace-nowrap transition after:absolute after:-inset-x-1 after:-inset-y-0.75 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {by
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

{#snippet status()}
	{#if count > 0}
		<span class="text-text-muted">{m.ticks_count({ count })}&nbsp;·</span>
		<button type="button" onclick={onclear} class="{link} font-semibold text-text-muted">
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
	The row of ticks under the search (spec, "Tick row"). On a phone a header
	line over one sideways line of pills, all of them; on a desktop the label
	left of wrapping pills, capped at CAP plus every ticked one until
	"+N more". Focused on a card it lists only that card's missing items and
	"Done", which sits in the header on a phone and at the right on a desktop:
	a two-column grid below `md`, a flex row from there with Done last.
-->
<div
	class="grid grid-cols-[minmax(0,1fr)_auto] items-center md:flex md:items-start {focus
		? 'gap-x-3'
		: 'md:gap-x-3'}"
>
	<div
		class="flex min-h-10 min-w-0 flex-wrap items-center gap-x-1.5 text-body-sm md:shrink-0 md:flex-nowrap"
	>
		{#if focus}
			<span id="{id}-label" class="font-bold">
				{focusText[0]}<em class="font-display text-body font-semibold text-primary italic"
					>{focus.title}</em
				>{focusText[1]}
			</span>
		{:else}
			<strong id="{id}-label">{m.ticks_heading()}</strong>
			<span class="contents md:hidden">
				{#if count > 0}<span aria-hidden="true" class="text-text-muted">·</span>{/if}
				{#if count > 0 || hint}<span
						class="flex items-center gap-x-1.5 text-caption {count > 0 ? '' : 'basis-full pb-1'}"
						>{@render status()}</span
					>{/if}
			</span>
		{/if}
	</div>

	{#if focus}
		<!-- 44px on a phone, a dense row's height; 40px beside the pills on a desktop. -->
		<Button onclick={ondone} class="gap-1.5 font-bold max-md:h-11 md:order-last md:shadow-cta">
			<Check class="size-4" strokeWidth={3} aria-hidden="true" />{m.ticks_done()}
		</Button>
		{#if items.length > 0}<div
				role="group"
				aria-labelledby="{id}-label"
				{@attach measure}
				class="col-span-2 -m-1 mt-1 flex min-w-0 flex-wrap gap-1.5 p-1 md:-mt-1 md:flex-1 {capped
					? 'max-h-51 overflow-y-auto overscroll-contain'
					: ''} {capped && !atEnd ? 'mask-b-from-[calc(100%-18px)]' : ''}"
			>
				{#each items as item (item.key)}{@render pill(item)}{/each}
			</div>{/if}
	{:else}
		<ScrollFade
			frame="col-span-2 -m-1 min-w-0 md:flex-1"
			fade="from-background"
			class="flex [scrollbar-width:none] gap-1.5 overflow-x-auto p-1 md:flex-wrap md:overflow-visible"
		>
			<!-- `contents`: the pills and the status after them wrap as one flow,
			     while the group holds only the pills. -->
			<div role="group" aria-labelledby="{id}-label" class="contents">
				{#each items as item (item.key)}{@render pill(item)}{/each}
				{#if hidden.size > 0}
					<button
						type="button"
						onclick={() => (expandedFor = row)}
						class="{link} hidden text-body-sm font-semibold text-text md:inline-flex"
					>
						{m.ticks_more({ count: hidden.size })}
					</button>
				{/if}
			</div>
			<span class="hidden min-h-10 items-center gap-x-1.5 pl-1 text-body-sm md:flex">
				{@render status()}
			</span>
		</ScrollFade>
	{/if}
</div>
