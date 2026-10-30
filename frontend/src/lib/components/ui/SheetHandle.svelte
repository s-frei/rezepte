<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { prefersReducedMotion } from 'svelte/motion';

	// The handle at the top of a phone's bottom sheet: a tap closes the sheet,
	// and so does pulling it down. The sheet follows the finger and closes
	// when let go a quarter of its height down or flicked; short of that it
	// springs back. The sheet is the handle's dialog, so callers pass nothing
	// but how to close it.
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

	/** Pixels a finger may wander before a press counts as a pull, not a tap. */
	const SLOP = 4;
	/** Downward speed, in px/ms, that closes the sheet however short the pull. */
	const FLICK = 0.5;

	let sheet: HTMLElement | null = null;
	let startY = 0;
	let lastY = 0;
	let lastTime = 0;
	let speed = 0;
	let pulled = false;

	function onpointerdown(event: PointerEvent) {
		if (event.button !== 0) return;
		sheet = (event.currentTarget as HTMLElement).closest('[role="dialog"]');
		if (!sheet) return;
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
		startY = lastY = event.clientY;
		lastTime = event.timeStamp;
		speed = 0;
		pulled = false;
		sheet.style.transition = 'none';
	}

	function onpointermove(event: PointerEvent) {
		if (!sheet) return;
		const dy = Math.max(0, event.clientY - startY);
		if (dy > SLOP) pulled = true;
		const dt = event.timeStamp - lastTime;
		if (dt > 0) speed = (event.clientY - lastY) / dt;
		lastY = event.clientY;
		lastTime = event.timeStamp;
		sheet.style.transform = `translateY(${dy}px)`;
	}

	function onpointerup() {
		if (!sheet) return;
		const node = sheet;
		sheet = null;
		// The click that follows a pull comes before this; a later one is a tap.
		setTimeout(() => (pulled = false));
		const dy = Math.max(0, lastY - startY);
		if (pulled && (dy > node.offsetHeight / 4 || speed > FLICK)) {
			// The sheet's own outro carries on from where the finger left it.
			node.style.transition = '';
			onclose();
			return;
		}
		node.style.transition = prefersReducedMotion.current ? '' : 'transform 200ms ease-out';
		node.style.transform = '';
		node.addEventListener('transitionend', () => (node.style.transition = ''), { once: true });
	}

	// A pull ends in a click on the handle; only a tap may close through it.
	function onclickcapture(event: MouseEvent) {
		if (!pulled) return;
		event.preventDefault();
		event.stopImmediatePropagation();
	}
</script>

<Dialog.Close>
	{#snippet child({ props })}
		<button
			{...props}
			aria-label={label}
			class="flex shrink-0 touch-none justify-center pt-2 pb-3 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary {className}"
			{onpointerdown}
			{onpointermove}
			{onpointerup}
			onpointercancel={onpointerup}
			{onclickcapture}
		>
			<span class="h-1 w-9 rounded-pill bg-handle" aria-hidden="true"></span>
		</button>
	{/snippet}
</Dialog.Close>
