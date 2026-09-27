<script lang="ts">
	import type { Snippet } from 'svelte';

	// Layout only - the caller decides what goes in each cell (real
	// `RecipeCard`s or skeleton placeholders while loading), so both share
	// the exact same grid.
	let { children }: { children: Snippet } = $props();
</script>

<!-- From `md` up a card carries the star and the tasty heart over its photo,
     and the column, not the card, guarantees them room: a track is never
     narrower than 220px, so a window just past `md` gets three columns
     instead of four 160px ones hidden under their controls, and never wider
     than a quarter of the row, so a wide window still stops at four.
     `auto-fill` rather than `auto-fit`: one search hit keeps a card's width
     instead of stretching over the whole row. -->
<div
	class="grid grid-cols-2 gap-3.5 md:grid-cols-[repeat(auto-fill,minmax(max(220px,calc((100%_-_3*22px)/4)),1fr))] md:gap-[22px]"
>
	{@render children()}
</div>
