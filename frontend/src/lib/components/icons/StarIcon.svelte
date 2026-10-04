<script lang="ts">
	// Lucide's star that turns and springs back once on `play()`: the caller
	// plays it when a tap sets the favorite, so it stays still otherwise.
	let { class: className = '' }: { class?: string } = $props();

	let spinning = $state(false);

	export function play() {
		spinning = true;
	}
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
	class:spinning
	onanimationend={() => (spinning = false)}
>
	<path
		d="M11.525 2.295a.53.53 0 0 1 .95 0l2.31 4.679a2.123 2.123 0 0 0 1.595 1.16l5.166.756a.53.53 0 0 1 .294.904l-3.736 3.638a2.123 2.123 0 0 0-.611 1.878l.882 5.14a.53.53 0 0 1-.771.56l-4.618-2.428a2.122 2.122 0 0 0-1.973 0L6.396 21.01a.53.53 0 0 1-.77-.56l.881-5.139a2.122 2.122 0 0 0-.611-1.879L2.16 9.795a.53.53 0 0 1 .294-.906l5.165-.755a2.122 2.122 0 0 0 1.597-1.16z"
	/>
</svg>

<style>
	svg {
		transform-origin: center;
	}

	@media (prefers-reduced-motion: no-preference) {
		.spinning {
			animation: spin 560ms var(--ease-spring);
		}
	}

	/* A fifth of a turn lands the star on its own outline again. */
	@keyframes spin {
		0% {
			rotate: 0deg;
			scale: 1;
		}
		50% {
			scale: 1.25;
		}
		100% {
			rotate: 72deg;
			scale: 1;
		}
	}
</style>
