<script lang="ts">
	import Maximize2 from '@lucide/svelte/icons/maximize-2';
	import { m } from '#lib/paraglide/messages.js';

	// "Photo n of m" and "View large" under a step's photo, in the card, the
	// sheet and cook mode's panel. The caller sets the row's own layout.
	let {
		index,
		count,
		onenlarge
	}: { index: number; count: number; onenlarge: (index: number) => void } = $props();
</script>

<span class="text-text-muted">{m.step_photo_position({ number: index + 1, count })}</span>
<button
	type="button"
	onclick={() => onenlarge(index)}
	class="inline-flex items-center gap-1.5 rounded-pill font-semibold text-text underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
>
	<Maximize2 class="size-4" aria-hidden="true" />
	{m.step_photo_enlarge()}
</button>
