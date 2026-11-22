<script lang="ts">
	import type { FindPart } from '#lib/api/recipes.js';

	let { items }: { items: { parts: FindPart[]; long?: boolean }[] } = $props();
</script>

<!-- Items joined by ", ", each search hit marked in weight and color, not a
     background, which a line cut short would leave behind as an empty patch
     at the ellipsis. A <b>, not a <mark>: VoiceOver announces every mark,
     and a hit inside a word ("Toast" + "brot") split it into three. A long
     item may hyphenate, and only that item, its comma outside the break;
     one with no hyphenation point in the locale's dictionary still breaks.
     The separator is a mustache: Svelte trims a literal ", " at the edge of
     a block to ",", which would run the items together. -->
{#snippet parts(item: { parts: FindPart[] })}{#each item.parts as part, k (k)}{#if part.hit}<b
				class="font-bold text-accent-foreground">{part.text}</b
			>{:else}{part.text}{/if}{/each}{/snippet}
<!-- eslint-disable-next-line svelte/no-useless-mustaches -->
{#each items as item, j (j)}{#if j > 0}{', '}{/if}{#if item.long}<span
			class="wrap-break-word hyphens-auto">{@render parts(item)}</span
		>{:else}{@render parts(item)}{/if}{/each}
