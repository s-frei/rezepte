<script lang="ts">
	// Lucide's circle-check that draws itself as it appears: the circle from
	// the top, then the check from its short stroke. It belongs to success
	// messages, which mount when the thing succeeded, so arriving is the cue.
	let { class: className = '' }: { class?: string } = $props();
</script>

<svg
	xmlns="http://www.w3.org/2000/svg"
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="2"
	stroke-linecap="round"
	stroke-linejoin="round"
	aria-hidden="true"
	class={className}
>
	<circle cx="12" cy="12" r="10" pathLength="1" transform="rotate(-90 12 12)" />
	<path d="M8 12l2.5 2.5L16 9" pathLength="1" />
</svg>

<style>
	circle,
	path {
		stroke-dasharray: 1;
	}

	@media (prefers-reduced-motion: no-preference) {
		circle {
			animation: draw 420ms ease-out both;
		}

		path {
			animation: draw 240ms ease-out 380ms both;
		}
	}

	@keyframes draw {
		from {
			stroke-dashoffset: 1;
		}
		to {
			stroke-dashoffset: 0;
		}
	}
</style>
