<script lang="ts">
	import PersonMark from '#lib/components/ui/PersonMark.svelte';

	type Person = { id: string; displayName: string; color: string; avatarId?: string | null };

	let {
		people,
		slots,
		size,
		ring,
		class: className = ''
	}: {
		people: Person[];
		/**
		 * Circles at most, the count included: up to `slots` people show as
		 * faces, beyond that the last slot turns into "+N". The stack never
		 * grows past this width, whatever the count.
		 */
		slots: number;
		size: 'xs' | 'sm';
		/** The ring in the ground's color (`ring-surface`), which cuts each circle from the one before. */
		ring: string;
		/** Placement only. */
		class?: string;
	} = $props();

	const faces = $derived(people.length <= slots ? people : people.slice(0, slots - 1));
	const more = $derived(people.length - faces.length);
	// The overlap plus the 2px ring covers under a third of the circle before
	// (6 of 20px, 8 of 28px), so the face under the next one keeps its
	// whole initial; at half, the "+N" cut the letter beside it.
	const overlap = $derived(size === 'xs' ? '-ml-1' : '-ml-1.5');
	const count = $derived(size === 'xs' ? 'h-5 min-w-5' : 'h-7 min-w-7');
</script>

<!-- Decoration: the text beside it always says who or how many. -->
<span aria-hidden="true" class="inline-flex {className}">
	{#each faces as person, index (person.id)}
		<PersonMark {person} {size} class="ring-2 {ring} {index > 0 ? overlap : ''}" />
	{/each}
	{#if more > 0}
		<!-- `relative` like PersonMark, so the count lies on top of the faces. -->
		<span
			class="relative {overlap} {count} flex items-center justify-center rounded-full border border-border bg-surface px-0.5 text-micro font-semibold text-text-muted ring-2 {ring}"
			>+{more}</span
		>
	{/if}
</span>
