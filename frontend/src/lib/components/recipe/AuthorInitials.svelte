<script lang="ts">
	import { Popover } from 'bits-ui';
	import { m } from '$lib/paraglide/messages';
	import type { Person } from '$lib/api/recipes';
	import { authorLabel } from '$lib/recipe/authorship';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';

	let {
		createdBy,
		updatedBy,
		anchor = null
	}: {
		createdBy: Person;
		updatedBy: Person;
		/** What the panel lines up with. The card, so the panel spans its
		 * full width and sits under it, rather than the two small circles
		 * that trigger it. */
		anchor?: HTMLElement | null;
	} = $props();

	// One circle per person involved, the author first. A recipe its own
	// author last edited needs no second circle - it would repeat the first.
	// Compared by id: a display name is deliberately not unique, two members
	// may both be "Mia".
	const sameAuthor = $derived(createdBy.id === updatedBy.id);
	const people = $derived(sameAuthor ? [createdBy] : [createdBy, updatedBy]);
	const edited = $derived(!sameAuthor);
	const label = $derived(authorLabel(createdBy, updatedBy));
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
		{#each people as person, index (person.id)}
			<PersonMark {person} size="xs" class="ring-2 ring-surface {index > 0 ? '-ml-1.5' : ''}" />
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
			here - it is the card's own color, so in dark mode the panel
			dissolved into the card it hangs from, and even in light mode a
			panel sharing the card's width, edge and color family read as a
			second piece of the card rather than as an overlay.

			The radius stays `rounded-2xl`, like every other floating panel in
			the app: with the color doing the separating, the shape no longer
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
			<p>{m.card_author({ user: createdBy.displayName })}</p>
			{#if edited}
				<p class="mt-1 text-inverse-muted">{m.card_editor({ user: updatedBy.displayName })}</p>
			{/if}
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>
