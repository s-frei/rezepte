<script lang="ts">
	import type { ListPerson } from '#lib/api/shopping.js';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';

	let { people }: { people: ListPerson[] } = $props();
</script>

<span role="img" aria-label={people.map((p) => p.displayName).join(', ')} class="flex shrink-0">
	{#each people as person, i (person.id)}
		<PersonMark {person} size="sm" class="ring-2 ring-surface {i > 0 ? '-ml-2' : ''}" />
	{/each}
</span>
