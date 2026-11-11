<script lang="ts">
	import { pullToClose } from './pull-to-close.js';

	// The handle at the top of a phone's bottom sheet (and the login's sheet):
	// a tap closes the sheet, and so does pulling it down (pullToClose).
	// Callers pass nothing but how to close it. The button is 44px tall, a
	// thumb's target, with the 4px grip in its middle; at its old 24px it was
	// easy to miss.
	let {
		label,
		onclose,
		class: className = ''
	}: {
		/** Accessible name of the handle. */
		label: string;
		onclose: () => void;
		class?: string;
	} = $props();
</script>

<button
	type="button"
	aria-label={label}
	onclick={onclose}
	class="flex h-11 shrink-0 touch-none items-center justify-center focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary {className}"
	{@attach pullToClose(onclose)}
>
	<span class="h-1 w-9 rounded-pill bg-handle" aria-hidden="true"></span>
</button>
