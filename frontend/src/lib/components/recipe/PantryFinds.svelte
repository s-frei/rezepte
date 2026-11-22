<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import List from '@lucide/svelte/icons/list';
	import PantryLine from './PantryLine.svelte';
	import type { PantryItem, SearchFind } from '#lib/api/recipes.js';
	import { m } from '#lib/paraglide/messages.js';
	import { itemText, missingParts } from '#lib/recipe/finds.js';

	let {
		used,
		missing,
		finds = [],
		photo = false,
		highlightKey = null
	}: {
		/** The recipe's own names for what the chips cover. */
		used: string[];
		/** What the recipe still needs, after the ticks. */
		missing: PantryItem[];
		/** The card's text finds: a word found in an ingredient named here is marked in its line. */
		finds?: SearchFind[];
		photo?: boolean;
		/** The ingredient the tick row's hovered pill names, marked in the missing line. */
		highlightKey?: string | null;
	} = $props();

	const hits = $derived(
		new Map(
			finds
				.filter((find) => find.field === 'ingredients')
				.flatMap((find) => find.items)
				.map((item) => [itemText(item), item.parts])
		)
	);
	const parts = $derived(missingParts(missing, hits, highlightKey));

	const tone = $derived(photo ? 'text-text-muted' : 'text-text/75');
	const label = $derived(
		`sr-only items-center gap-1 text-micro font-medium desk:not-sr-only desk:flex ${tone}`
	);
</script>

<!-- What the recipe has of the reader's ingredients and what it still
     needs, in `SearchFinds`' look: a rule and a label row over each line.
     A two-column phone card keeps the rule and the label's icon and drops
     only its word, which screen readers still hear. The rule hangs in the
     card's padding, so the lines keep the title's edge and full width.
     Both lines wrap and end in "+N" (`PantryLine`): the reader's own
     ingredients take two lines at most, marked like a search hit, the
     missing ones three. -->
<ul class="mt-1.5 flex flex-col gap-1.5 desk:mt-3 desk:gap-2">
	{#if used.length > 0}
		<li
			class="-ml-2 flex min-w-0 flex-col border-l-2 border-primary/45 pl-1.5 desk:ml-0 desk:gap-0.5 desk:py-px desk:pl-2.5"
		>
			<span class={label}>
				<Check class="size-3.5 shrink-0 text-success-foreground" aria-hidden="true" />
				<span class="truncate">{m.pantry_yours()}<span class="sr-only">:</span></span>
			</span>
			<PantryLine
				data-used
				lines={2}
				items={used.map((name) => ({ parts: [{ text: name, hit: true }] }))}
			>
				{#snippet lead()}<Check
						class="mr-1 inline-block size-3.5 stroke-[2.6] align-[-0.2em] text-success-foreground desk:hidden"
						aria-hidden="true"
					/>{/snippet}
			</PantryLine>
		</li>
	{/if}
	<li
		class="-ml-2 flex min-w-0 flex-col border-l-2 border-handle pl-1.5 desk:ml-0 desk:gap-0.5 desk:py-px desk:pl-2.5"
	>
		<span class={label}>
			<List class="size-3.5 shrink-0" aria-hidden="true" />
			<span class="truncate">{m.pantry_missing_label()}<span class="sr-only">:</span></span>
		</span>
		<PantryLine data-missing lines={3} items={parts}>
			{#snippet lead()}<List
					class="mr-1 inline-block size-3.5 stroke-[2.6] align-[-0.2em] text-text/75 desk:hidden"
					aria-hidden="true"
				/>{/snippet}
			{#snippet empty()}<span class="text-text-muted">{m.pantry_nothing_missing()}</span>{/snippet}
		</PantryLine>
	</li>
</ul>
