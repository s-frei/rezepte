<script lang="ts">
	import { Popover } from 'bits-ui';
	import { m } from '$lib/paraglide/messages';
	import { authorLabel } from '$lib/recipe/authorship';
	import { tintFor, type Tint } from '$lib/recipe/placeholder';

	let {
		createdByName,
		updatedByName,
		anchor = null
	}: {
		createdByName: string;
		updatedByName: string;
		/** What the panel lines up with. The card, so the panel spans its
		 * full width and sits under it, rather than the two small circles
		 * that trigger it. */
		anchor?: HTMLElement | null;
	} = $props();

	const TINT_CLASSES: Record<Tint, string> = {
		'tint-1': 'bg-tint-1',
		'tint-2': 'bg-tint-2',
		'tint-3': 'bg-tint-3'
	};

	// One circle per person involved, the author first. A recipe its own
	// author last edited needs no second circle - it would repeat the first.
	const names = $derived(
		createdByName === updatedByName ? [createdByName] : [createdByName, updatedByName]
	);
	const edited = $derived(createdByName !== updatedByName);
	const label = $derived(authorLabel(createdByName, updatedByName));
</script>

<!--
	A popover rather than a tooltip, because bits-ui's tooltip returns early
	on `pointerType === "touch"` and closes rather than opens on click - it
	is hover-only by design, which would leave phones with two unexplained
	letters. A popover opens on tap everywhere and, with `openOnHover`,
	still behaves like a tooltip under a mouse.
-->
<Popover.Root>
	<Popover.Trigger
		openOnHover
		openDelay={300}
		aria-label={label}
		tabindex={-1}
		class="flex shrink-0 items-center rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
	>
		{#each names as name, index (name)}
			<!--
				`relative` is what makes the overlap read as one circle in front
				of another. Without it these are unpositioned boxes, and CSS
				paints every background first and every piece of text
				afterwards - so the editor's circle covered the author's circle
				but not the author's letter, which went on floating over the
				gap. Positioned elements are painted whole, in document order.
			-->
			<span
				aria-hidden="true"
				class="relative flex size-5 items-center justify-center rounded-full font-display text-micro leading-none font-semibold text-primary ring-2 ring-surface {TINT_CLASSES[
					tintFor(name)
				]} {index > 0 ? '-ml-1.5' : ''}"
			>
				<!--
					`items-center` centres the line box, not the letter. A capital
					has no descender, so the descender space the font reserves
					below the baseline pushes the visible glyph down off centre.
					How far is a property of the font alone - the circle's size
					and the line height both cancel out of the arithmetic:

					    nudge = (inkAscent + fontDescent - fontAscent) / 2

					which for Literata at 12px (8.41, 4, 14 - measured through
					canvas `measureText`) is -0.79px. Rounded to 0.8; anything
					coarser is visible at this size, as a 1px nudge the wrong way
					proved.
				-->
				<span class="-translate-y-[0.8px]">{name.charAt(0).toUpperCase()}</span>
			</span>
		{/each}
	</Popover.Trigger>
	<Popover.Portal>
		<!--
			The card is the anchor, not the trigger: the panel then hangs below
			the card and spans exactly its width (`--bits-floating-anchor-width`
			is the anchor's measured width), which reads as belonging to that
			card rather than pointing at two small circles inside it.

			`inverse`, the same pair toasts and the bottom nav use: it is the
			palette's "something laid over the page" role, and it inverts with
			the theme by itself. The dropdowns' `surface` cannot do that job
			here - it is the card's own colour, so in dark mode the panel
			dissolved into the card it hangs from, and even in light mode a
			panel sharing the card's width, edge and colour family read as a
			second piece of the card rather than as an overlay.

			The radius stays `rounded-2xl`, like every other floating panel in
			the app: with the colour doing the separating, the shape no longer
			has to, and a flatter corner here would only make this one panel
			the odd shape out.

			`collisionPadding` keeps it off the screen edge when a card sits at
			the very bottom of the viewport and Floating UI flips the panel
			back above it.
		-->
		<Popover.Content
			customAnchor={anchor}
			side="bottom"
			sideOffset={6}
			align="start"
			collisionPadding={12}
			class="z-50 w-[var(--bits-floating-anchor-width)] rounded-2xl bg-inverse px-3.5 py-2.5 text-caption text-inverse-foreground shadow-dialog"
		>
			<p>{m.card_author({ user: createdByName })}</p>
			{#if edited}
				<p class="mt-1 text-inverse-muted">{m.card_editor({ user: updatedByName })}</p>
			{/if}
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>
