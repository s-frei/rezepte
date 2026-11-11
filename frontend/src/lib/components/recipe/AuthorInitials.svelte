<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { m } from '#lib/paraglide/messages.js';
	import type { Person } from '#lib/api/recipes.js';
	import { authorLabel } from '#lib/recipe/authorship.js';
	import PersonCard from '#lib/components/ui/PersonCard.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import PersonPeek from '#lib/components/ui/PersonPeek.svelte';

	let {
		createdBy,
		updatedBy,
		anchor = null,
		fresh = false
	}: {
		createdBy: Person;
		updatedBy: Person;
		/** What the panel lines up with. The card, so the panel spans its
		 * full width and sits under it, rather than the two small circles
		 * that trigger it. */
		anchor?: HTMLElement | null;
		/** Unseen diary entries: a dot on the circles, and the label says so. */
		fresh?: boolean;
	} = $props();

	// One circle per person involved, the author first. A recipe its own
	// author last edited needs no second circle - it would repeat the first.
	// Compared by id: a display name is deliberately not unique, two members
	// may both be "Mia".
	const sameAuthor = $derived(createdBy.id === updatedBy.id);
	const people = $derived(sameAuthor ? [createdBy] : [createdBy, updatedBy]);
	const edited = $derived(!sameAuthor);
	const label = $derived(
		fresh
			? m.card_new_entries({ label: authorLabel(createdBy, updatedBy) })
			: authorLabel(createdBy, updatedBy)
	);
	// Below `md` the people open in a bottom sheet (PersonPeek): under a card
	// in a two-column grid the panel was 153px wide, cut the name and the
	// role off and slid under the bottom nav.
	let open = $state(false);
</script>

<!-- Two cards each carry their label above them; one card needs none. -->
{#snippet cards(muted: string, compact: boolean, inverse = false)}
	<div class="space-y-3">
		{#if edited}
			<p class={muted}>{m.card_author({ user: createdBy.displayName })}</p>
		{/if}
		<PersonCard person={createdBy} {compact} {inverse} onnavigate={() => (open = false)} />
		{#if edited}
			<p class={muted}>{m.card_editor({ user: updatedBy.displayName })}</p>
			<PersonCard person={updatedBy} {compact} {inverse} onnavigate={() => (open = false)} />
		{/if}
	</div>
{/snippet}

<!-- The new-entries dot is positioned on the trigger, which is `relative`,
	so the circles stay its first children with no wrapper around them.
	Each size is the card corner's: the circles' center is the center of the
	card's bottom-right corner, so 24px in a phone card's 24px corner at a
	12px gap, 28px in the desktop card's 28px corner at 14px (see RecipeCard).
	`desk:`, not the screen width: a phone showing one card per row shows the
	desktop card. -->
{#snippet circles()}
	{#each people as person, index (person.id)}
		<PersonMark {person} size="corner" class="ring-2 ring-surface {index > 0 ? '-ml-1.5' : ''}" />
	{/each}
	{#if fresh}
		<span
			aria-hidden="true"
			class="absolute -top-0.5 -right-0.5 size-2 rounded-full bg-primary ring-2 ring-surface"
		></span>
	{/if}
{/snippet}

<!--
	The card is the popover's anchor, not the trigger: the panel then hangs
	below the card and spans exactly its width (`--bits-floating-anchor-width`
	is the anchor's measured width), which reads as belonging to that card
	rather than pointing at two small circles inside it.

	`inverse`, the same pair toasts use: it is the palette's "something laid
	over the page" role, and it inverts with the theme by itself. The
	dropdowns' `surface` cannot do that job here - it is the card's own
	color, so in dark mode the panel dissolved into the card it hangs from,
	and even in light mode a panel sharing the card's width, edge and color
	family read as a second piece of the card rather than as an overlay.

	The radius stays `rounded-2xl`, like every other floating panel in the
	app: with the color doing the separating, the shape no longer has to.
-->
<PersonPeek
	bind:open
	{label}
	{anchor}
	class="relative flex shrink-0 items-center rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
	panelClass="w-[var(--bits-floating-anchor-width)] bg-inverse p-3.5 text-caption text-inverse-foreground"
	trigger={circles}
>
	{#snippet card(sheet)}
		{#if sheet}
			<Dialog.Title class="sr-only">{label}</Dialog.Title>
			{@render cards('text-caption text-text-muted', false)}
		{:else}
			{@render cards('text-inverse-muted', true, true)}
		{/if}
	{/snippet}
</PersonPeek>
