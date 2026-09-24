<script lang="ts">
	import { untrack } from 'svelte';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import type { Tag } from '$lib/api/recipes';
	import TagChip from '$lib/components/ui/TagChip.svelte';
	import { m } from '$lib/paraglide/messages';

	let {
		tags,
		active,
		ontoggle
	}: {
		/** All tags currently in use, from `listTags()`. */
		tags: Tag[];
		/** Currently selected tag names. */
		active: string[];
		ontoggle: (name: string) => void;
	} = $props();

	const GAP_PX = 8; // gap-2

	// This row is the only place tags are filtered from. An earlier version
	// listed them a second time inside the filter panel, with the overflow
	// button opening that panel - two ways to set the same filter, one of
	// them behind a button that also holds time and favorites. Overflowing
	// tags now stay here: the button unfolds this row into as many lines as
	// it takes and folds it back again.
	let expanded = $state(false);

	let row = $state<HTMLDivElement | null>(null);
	/** Off-screen twin of `row` holding every chip, for width measurement. */
	let measureRow = $state<HTMLDivElement | null>(null);
	/** The "+N weitere" button's own width, measured off-screen (see below). */
	let moreButton = $state<HTMLButtonElement | null>(null);
	/**
	 * Names the folded row shows; all of them until `measure()` has run.
	 *
	 * An array rather than a `Set`, which this would otherwise be: the lint
	 * rule `svelte/prefer-svelte-reactivity` rejects a built-in `Set` in a
	 * component, and a `SvelteSet` would only buy reactivity on mutation,
	 * which nothing here does - `measure()` builds a fresh list and assigns
	 * it. At the couple of dozen tags a household collects, the difference
	 * between `includes` and `has` is not measurable.
	 */
	let folded = $state<string[]>(untrack(() => tags.map((tag) => tag.name)));

	const shown = $derived(expanded ? tags : tags.filter((tag) => folded.includes(tag.name)));
	const hiddenCount = $derived(tags.length - folded.length);

	function registerRow(node: HTMLDivElement) {
		row = node;
		return () => {
			row = null;
		};
	}

	function registerMeasureRow(node: HTMLDivElement) {
		measureRow = node;
		return () => {
			measureRow = null;
		};
	}

	function registerMoreButton(node: HTMLButtonElement) {
		moreButton = node;
		return () => {
			moreButton = null;
		};
	}

	// One line of chips, measured rather than guessed: chip widths depend on
	// the tag names and the font, so no breakpoint can predict the count.
	// Measuring keeps running while the row is unfolded - `row` keeps its
	// width there, only its height grows - so folding it back lands on a
	// count that still fits.
	//
	// The folded row renders only the chips that fit, so it can never grow
	// wider than its container. Working that out still needs every chip's
	// width, including the ones that will not be shown -- an unmounted chip
	// has no width to measure -- so a second, off-screen row (`measureRow`,
	// see the markup) always renders the full `tags` list purely to be
	// measured. `row` itself never holds an invisible chip, which matters:
	// an early version kept hidden chips in `row` with `visibility: hidden`,
	// and because they still occupied flex space, they pushed the "+N"
	// button that followed them off the end of the clipped row -- correctly
	// sized and "visible" by computed style, permanently invisible in
	// practice. Splitting measurement from display removes that trap.
	//
	// The "+N weitere" button has the same chicken-and-egg problem on its
	// own: its width can't come from its own in-flow instance, because that
	// instance only exists once we already know some chips are hidden.
	// Instead an invisible, out-of-flow copy of the button is always
	// rendered (below), sized off `tags.length` -- a stable upper bound on
	// the digit count any real hidden-count can ever need -- so a width is
	// always available before the real button ever has a reason to exist.
	function measure() {
		if (!row || !measureRow) {
			return;
		}
		const chips = [...measureRow.children] as HTMLElement[];
		if (chips.length !== tags.length) {
			// The DOM hasn't caught up with a `tags` change yet; the next
			// mutation (or the resize it likely causes) re-triggers this.
			return;
		}

		const rowWidth = row.clientWidth;
		const totalWidth = chips.reduce(
			(sum, chip, i) => sum + chip.offsetWidth + (i > 0 ? GAP_PX : 0),
			0
		);

		// Everything fits on its own: show every chip, reserve nothing for a
		// button that would only waste the space it sits in.
		if (totalWidth <= rowWidth) {
			folded = tags.map((tag) => tag.name);
			return;
		}

		const buttonWidth = moreButton?.offsetWidth ?? 0;
		const limit = rowWidth - buttonWidth - GAP_PX;
		const chosen: string[] = [];
		let used = 0;

		// Chips are rendered in `tags` order whatever order they were picked
		// in, and n chips always cost n-1 gaps, so `used` holds for any
		// selection - which is what lets the two passes below pick out of
		// order without having to care where a chip will end up.
		function take(index: number) {
			const width = chips[index].offsetWidth;
			const next = used === 0 ? width : used + GAP_PX + width;
			if (next > limit) {
				return false;
			}
			used = next;
			chosen.push(tags[index].name);
			return true;
		}

		// Active tags first: a filter that is on has to stay visible, or the
		// row goes quiet about the very thing narrowing the grid below it -
		// the count on the "Filter" button would say one is set and nothing
		// on screen would say which. They are taken in order and individually,
		// so a wide one that does not fit does not take a narrower one after
		// it down with it.
		for (let i = 0; i < tags.length; i += 1) {
			if (active.includes(tags[i].name)) {
				take(i);
			}
		}
		// Then the rest, stopping at the first that does not fit rather than
		// skipping it for a narrower one behind it: a row that reads as "the
		// first few, then the ones you picked" is one a reader can follow,
		// where an arbitrary subset is not.
		for (let i = 0; i < tags.length; i += 1) {
			if (!chosen.includes(tags[i].name) && !take(i)) {
				break;
			}
		}
		folded = chosen;
	}

	$effect(() => {
		// Re-measure when the tag list changes, not only on resize.
		void tags.length;
		// And when the selection changes: which chips the folded row keeps
		// depends on which of them are active. `measure()` reads `active`
		// itself, but only past the guards above, so the dependency is named
		// here to hold even on the passes that return early.
		void active;
		if (!row) {
			return;
		}
		const observer = new ResizeObserver(() => measure());
		observer.observe(row);
		// Also watch the measuring row: a chip that widens for any reason
		// (a count crossing a digit boundary, a rename, a font swap) has to
		// trigger a re-measure too, not just a resize of `row` itself.
		// Reading `measureRow` here also means this effect re-runs once its
		// `{@attach}` sets it, in case it wasn't there yet on this pass.
		if (measureRow) {
			observer.observe(measureRow);
		}
		measure();
		return () => observer.disconnect();
	});
</script>

{#if tags.length > 0}
	<!-- Folded, the row is one clipped line (`overflow-hidden`) holding the
	     chips `measure()` found room for - always in `tags` order, however
	     they were picked. Unfolded, it wraps instead: the clip has to go, or
	     the lines below the first would be cut off. The toggle trails the last
	     chip in both states, so it reads as the end of the list rather than
	     moving somewhere else on the way. -->
	<div
		{@attach registerRow}
		class="flex min-w-0 flex-1 items-center gap-2 {expanded ? 'flex-wrap' : 'overflow-hidden'}"
	>
		{#each shown as tag (tag.name)}
			<span class="shrink-0">
				<TagChip
					label={tag.name}
					count={tag.count}
					active={active.includes(tag.name)}
					onclick={() => ontoggle(tag.name)}
				/>
			</span>
		{/each}
		<!-- Only when something is actually hidden: a row that fits needs no
		     toggle, and `expanded` left over from a narrower viewport then
		     changes nothing on screen. Same pill as a chip, in muted text and
		     without a count, so it belongs to the row without reading as a tag
		     of its own; the chevron is what says it unfolds something rather
		     than filtering by one more name, and it turns with the state. -->
		{#if hiddenCount > 0}
			<button
				type="button"
				onclick={() => (expanded = !expanded)}
				aria-expanded={expanded}
				class="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-pill border border-border bg-surface px-3.5 text-body-sm font-medium text-text-muted transition hover:text-text"
			>
				{expanded ? m.overview_tags_less() : m.overview_tags_more({ count: hiddenCount })}
				<ChevronDown
					class="size-4 transition-transform {expanded ? 'rotate-180' : ''}"
					aria-hidden="true"
				/>
			</button>
		{/if}
	</div>
	<!-- Off-screen measuring twin: every chip, laid out exactly like the real
	     ones (same interactive TagChip variant, same active state) so their
	     widths match, plus a stand-in "+N weitere" button. `absolute` is
	     what takes this second root element out of the parent's flex layout
	     and out of its scroll extents -- without it this row would compete
	     for space with `row` above instead of floating free of it; `h-0 w-0
	     overflow-hidden` then keeps its own (large) content from expanding
	     anything, while each child still lays out at its natural width,
	     which is all `measure()` reads from them. `inert` is what keeps this
	     twin out of the tab order and a real browser's accessibility tree --
	     it replaces `pointer-events-none` and a `tabindex="-1"` on every
	     chip, so don't reach for those instead: without it, every one of
	     these otherwise-identical chip buttons is a real, invisible tab
	     stop. `aria-hidden="true"` sits alongside it (not instead of it):
	     Playwright's own role-locator engine reimplements ARIA hiding rules
	     from the DOM rather than reading the real accessibility tree, and it
	     does not special-case `inert` (confirmed by a strict-mode violation
	     in the e2e suite once a chip in this twin shared its name with a
	     visible one) -- `aria-hidden` is a rule it does honor. -->
	<div class="absolute h-0 w-0 overflow-hidden" inert aria-hidden="true">
		<div {@attach registerMeasureRow} class="flex items-center gap-2">
			{#each tags as tag (tag.name)}
				<span class="shrink-0">
					<TagChip
						label={tag.name}
						count={tag.count}
						active={active.includes(tag.name)}
						onclick={() => {}}
					/>
				</span>
			{/each}
		</div>
		<!-- Every class the real toggle carries that adds width, the chevron
		     included: what `measure()` reserves has to be what the folded row
		     then puts there, or the button it made room for still overflows.

		     `w-max` is part of that. This button is a direct child of a
		     zero-width block, where an inline-level box shrinks to fit the
		     space it is offered - none - and settles on its min-content width:
		     its widest single word, not its whole label. It reported 16px less
		     than the real toggle needs, and the row's `overflow-hidden` then
		     clipped exactly that much off the right edge at 360px. The chips
		     above escape this because they sit in a flex row, where a flex
		     item keeps its natural width; the button has no such row, so it
		     has to be told. -->
		<button
			{@attach registerMoreButton}
			type="button"
			class="inline-flex h-9 w-max shrink-0 items-center gap-1.5 rounded-pill border border-border bg-surface px-3.5 text-body-sm font-medium text-text-muted"
		>
			{m.overview_tags_more({ count: tags.length })}
			<ChevronDown class="size-4" aria-hidden="true" />
		</button>
	</div>
{/if}
