<script lang="ts">
	import { Dialog, Popover } from 'bits-ui';
	import { MediaQuery } from 'svelte/reactivity';
	import { m } from '$lib/paraglide/messages';
	import type { Person } from '$lib/api/recipes';
	import { authorLabel } from '$lib/recipe/authorship';
	import BottomSheet from '$lib/components/ui/BottomSheet.svelte';
	import PersonCard from '$lib/components/ui/PersonCard.svelte';
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
	let open = $state(false);
	// Below `md` the people open in a bottom sheet instead: under a card in a
	// two-column grid the panel was 153px wide, cut the name and the role off
	// and slid under the bottom nav. The sheet is the phone's one shape for
	// "one person" (see PersonSheet).
	const phone = new MediaQuery('max-width: 767.98px');
</script>

<!-- Two cards each carry their label above them; one card needs none. -->
{#snippet cards(muted: string, compact: boolean)}
	<div class="space-y-3">
		{#if edited}
			<p class={muted}>{m.card_author({ user: createdBy.displayName })}</p>
		{/if}
		<PersonCard person={createdBy} {compact} onnavigate={() => (open = false)} />
		{#if edited}
			<p class={muted}>{m.card_editor({ user: updatedBy.displayName })}</p>
			<PersonCard person={updatedBy} {compact} onnavigate={() => (open = false)} />
		{/if}
	</div>
{/snippet}

{#snippet circles()}
	{#each people as person, index (person.id)}
		<PersonMark {person} size="xs" class="ring-2 ring-surface {index > 0 ? '-ml-1.5' : ''}" />
	{/each}
{/snippet}

<!--
	On a wide screen a popover rather than a tooltip, because bits-ui's
	tooltip returns early on `pointerType === "touch"` and closes rather than
	opens on click - a touch laptop would be left with two unexplained
	letters. A popover opens on tap everywhere and, with `openOnHover`,
	still behaves like a tooltip under a mouse.
-->
{#if phone.current}
	<button
		type="button"
		aria-label={label}
		onclick={() => (open = true)}
		class="flex shrink-0 items-center rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
	>
		{@render circles()}
	</button>
	<BottomSheet bind:open closeLabel={m.common_close()}>
		<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">
			<Dialog.Title class="sr-only">{label}</Dialog.Title>
			{@render cards('text-caption text-text-muted', false)}
		</div>
	</BottomSheet>
{:else}
	<Popover.Root bind:open>
		<Popover.Trigger
			openOnHover
			openDelay={300}
			aria-label={label}
			class="flex shrink-0 items-center rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			{@render circles()}
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
				class="z-50 w-[var(--bits-floating-anchor-width)] rounded-2xl bg-inverse p-3.5 text-caption text-inverse-foreground shadow-dialog"
			>
				{@render cards('text-inverse-muted', true)}
			</Popover.Content>
		</Popover.Portal>
	</Popover.Root>
{/if}
