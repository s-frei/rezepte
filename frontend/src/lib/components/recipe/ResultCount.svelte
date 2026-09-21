<script lang="ts">
	import { m } from '$lib/paraglide/messages';

	let {
		count,
		filtered
	}: {
		/** How many recipes the current query matches. */
		count: number;
		/** Whether a search term or any filter is active. */
		filtered: boolean;
	} = $props();
</script>

<!--
	Two keys plus a ternary rather than an ICU plural: this project's
	paraglide plugin does not parse ICU, and the codebase already picks
	plurals this way (see `servings_unit_one` in `format.ts`). "found" (like
	German "Treffer") reads the same for one result as for many, so the
	filtered line needs no singular variant.
-->
<p class="text-caption text-text-muted">
	{#if filtered}
		{m.overview_result_count_filtered({ count })}
	{:else if count === 1}
		{m.overview_result_count_one()}
	{:else}
		{m.overview_result_count({ count })}
	{/if}
</p>
