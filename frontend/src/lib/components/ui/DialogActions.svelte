<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		class: className = '',
		children
	}: {
		/** Placement only: the gap above, a hairline. */
		class?: string;
		children: Snippet;
	} = $props();
</script>

<!--
	The buttons at a dialog's foot, Cancel before the confirm. They wrap
	rather than squeeze: at 200% text on a 320px phone "Cancel" and "Send
	link" no longer fit side by side, and on one row they ran off the
	dialog's left edge. Below md each grows to fill its line and is 44px
	tall; from md they sit content-sized at the end.
-->
<div
	class="flex flex-wrap justify-end gap-3 *:shrink-0 *:grow *:whitespace-nowrap max-md:*:h-11 md:*:grow-0 {className}"
>
	{@render children()}
</div>
