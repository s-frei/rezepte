<script lang="ts">
	import { Popover } from 'bits-ui';
	import { m } from '$lib/paraglide/messages';
	import type { Person } from '$lib/api/recipes';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';

	let { people }: { people: Person[] } = $props();

	// A handful of circles says "these people" at a glance; past that they
	// would only grow into a row of letters nobody reads, so the rest are a
	// count and the names wait in the panel.
	const shown = 4;
	const circles = $derived(people.slice(0, shown));
	const more = $derived(people.length - circles.length);
	const names = $derived(people.map((person) => person.displayName).join(', '));
</script>

<!--
	A popover rather than a tooltip, for the reason AuthorInitials gives:
	bits-ui's tooltip ignores touch, and on a phone the names would then be
	out of reach.
-->
{#if people.length > 0}
	<Popover.Root>
		<Popover.Trigger
			openOnHover
			openDelay={300}
			aria-label={`${m.recipe_tasty_by()}: ${names}`}
			class="flex shrink-0 items-center rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			{#each circles as person, index (person.id)}
				<PersonMark {person} size="sm" class="ring-2 ring-background {index > 0 ? '-ml-2' : ''}" />
			{/each}
			{#if more > 0}
				<span aria-hidden="true" class="ml-1.5 text-caption text-text-muted">+{more}</span>
			{/if}
		</Popover.Trigger>
		<Popover.Portal>
			<!-- `inverse`, like the author panel: the palette's role for
			     something laid over the page. -->
			<Popover.Content
				side="bottom"
				sideOffset={6}
				align="start"
				collisionPadding={12}
				class="z-50 max-w-72 rounded-2xl bg-inverse px-3.5 py-2.5 text-caption text-inverse-foreground shadow-dialog"
			>
				<p class="text-inverse-muted">{m.recipe_tasty_by()}</p>
				<p class="mt-1">{names}</p>
			</Popover.Content>
		</Popover.Portal>
	</Popover.Root>
{/if}
