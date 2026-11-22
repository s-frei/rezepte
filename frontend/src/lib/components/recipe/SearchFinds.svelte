<script lang="ts">
	import FindParts from './FindParts.svelte';
	import type { SearchFind } from '#lib/api/recipes.js';
	import { m } from '#lib/paraglide/messages.js';
	import { findIcons, findLabel, findText } from '#lib/recipe/finds.js';

	let {
		finds,
		photo = false,
		pantry = false
	}: {
		finds: SearchFind[];
		photo?: boolean;
		/** Below the pantry lines, whose two-column phone look it takes: one
		 * line under a hanging rule, the field's icon in place of its label
		 * row. */
		pantry?: boolean;
	} = $props();
</script>

<!-- One find per field the words the title does not show were found in:
     the field as a quiet first line, what they were found by below, set off
     by a thin rule in the accent like a quotation rather than a box, which
     on a two-column phone card hangs in the card's padding, so the lines
     keep the title's edge. The field's name follows the card's tag line: muted
     on a photo card's paper, the text color dimmed on a text card's tint,
     where the muted gray misses AA. The find stays on one line; the server
     starts an excerpt at the word the match is in, so the ellipsis cuts
     after it, and the tooltip has the rest. -->
<ul
	class="flex flex-col {pantry ? 'mt-1.5 gap-1.5 desk:mt-2 desk:gap-2' : 'mt-2.5 gap-2 desk:mt-3'}"
>
	{#each finds as find, i (i)}
		{@const Icon = findIcons[find.field]}
		<li
			class="@container flex min-w-0 {pantry
				? '-ml-2 items-center gap-1 border-l-2 border-primary/45 pl-1.5 desk:ml-0 desk:flex-col desk:items-stretch desk:gap-0.5 desk:py-px desk:pl-2.5'
				: '-ml-2 flex-col gap-0.5 border-l-2 border-primary/45 py-px pl-1.5 desk:ml-0 desk:pl-2.5'}"
		>
			<!-- The count sits at the end of the field's line, where it costs
			     the find nothing; a screen reader hears it after the find. A
			     box too narrow for "Beschreibung" (a two-column card at 320px)
			     drops the icon before a letter of the name. -->
			{#if pantry}
				<Icon class="size-3.5 shrink-0 stroke-[2.6] text-text/75 desk:hidden" aria-hidden="true" />
			{/if}
			<span
				class="flex min-w-0 items-center gap-1 text-micro font-medium {pantry
					? 'sr-only desk:not-sr-only desk:flex'
					: ''} {photo ? 'text-text-muted' : 'text-text/75'}"
			>
				<Icon class="size-3.5 shrink-0 @max-[6.5rem]:hidden" aria-hidden="true" />
				<span class="truncate">{findLabel(find)}<span class="sr-only">:</span></span>
				{#if find.more > 0}
					<span aria-hidden="true" class="ml-auto shrink-0 tabular-nums">
						{m.search_find_more({ count: find.more })}
					</span>
				{/if}
			</span>
			<span class="truncate text-caption text-text" title={findText(find)}>
				<FindParts items={find.items} />{#if find.more > 0}<span class="sr-only"
						>, {m.search_find_more({ count: find.more })}</span
					>{/if}
			</span>
		</li>
	{/each}
</ul>
