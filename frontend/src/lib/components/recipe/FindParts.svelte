<script lang="ts">
	import type { FindPart } from '#lib/api/recipes.js';

	let { items }: { items: { parts: FindPart[] }[] } = $props();
</script>

<!-- Items joined by ", ", each search hit marked in weight and color, not a
     background, which a line cut short would leave behind as an empty patch
     at the ellipsis. A <b>, not a <mark>: VoiceOver announces every mark,
     and a hit inside a word ("Toast" + "brot") split it into three. The separator is a mustache: Svelte trims a literal
     ", " at the edge of a block to ",", which would run the items together. -->
<!-- eslint-disable-next-line svelte/no-useless-mustaches -->
{#each items as item, j (j)}{#if j > 0}{', '}{/if}{#each item.parts as part, k (k)}{#if part.hit}<b
				class="font-bold text-accent-foreground">{part.text}</b
			>{:else}{part.text}{/if}{/each}{/each}
