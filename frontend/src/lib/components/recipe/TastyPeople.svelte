<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { flip } from 'svelte/animate';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade } from 'svelte/transition';
	import { m } from '#lib/paraglide/messages.js';
	import type { Person } from '#lib/api/recipes.js';
	import PersonCard from '#lib/components/ui/PersonCard.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import PersonPeek from '#lib/components/ui/PersonPeek.svelte';

	let {
		people,
		changed = null
	}: {
		people: Person[];
		/** The person whose circle the reader just added or took away; only it moves. */
		changed?: string | null;
	} = $props();

	// A handful of circles says "these people" at a glance; past that they
	// would only grow into a row of letters nobody reads, so the rest are a
	// count and the names wait in the panel.
	const shown = 4;
	const circles = $derived(people.slice(0, shown));
	const more = $derived(people.length - circles.length);
	const names = $derived(people.map((person) => person.displayName).join(', '));
	let open = $state(false);
	// The changed circle pops in once the heart has beaten (`animate-pop`,
	// CSS so it plays when the stack itself is new) or fades out, and the
	// others make room; a new recipe's stack just appears.
	const room = $derived(changed && !prefersReducedMotion.current ? 200 : 0);
</script>

{#snippet faces()}
	{#each circles as person, index (person.id)}
		<span
			class="flex {index > 0 ? '-ml-2' : ''} {person.id === changed
				? 'motion-safe:animate-pop'
				: ''}"
			animate:flip={{ duration: room }}
			out:fade={{ duration: person.id === changed ? 120 : 0 }}
		>
			<PersonMark {person} size="sm" class="ring-2 ring-background" />
		</span>
	{/each}
	{#if more > 0}
		<span aria-hidden="true" class="ml-1.5 text-caption text-text-muted">+{more}</span>
	{/if}
{/snippet}

{#snippet cards(compact: boolean, inverse = false)}
	<ul class="mt-2 space-y-3">
		{#each people as person (person.id)}
			<li><PersonCard {person} {compact} {inverse} onnavigate={() => (open = false)} /></li>
		{/each}
	</ul>
{/snippet}

<!-- `inverse`, like the author panel: the palette's role for something
     laid over the page. -->
{#if people.length > 0}
	<PersonPeek
		bind:open
		label={`${m.recipe_tasty_by()}: ${names}`}
		class="flex shrink-0 items-center rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		panelClass="max-w-72 bg-inverse p-3.5 text-caption text-inverse-foreground"
		trigger={faces}
	>
		{#snippet card(sheet)}
			{#if sheet}
				<Dialog.Title class="text-caption text-text-muted">{m.recipe_tasty_by()}</Dialog.Title>
				{@render cards(false)}
			{:else}
				<p class="text-inverse-muted">{m.recipe_tasty_by()}</p>
				{@render cards(true, true)}
			{/if}
		{/snippet}
	</PersonPeek>
{/if}
