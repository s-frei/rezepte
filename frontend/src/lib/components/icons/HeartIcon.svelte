<script lang="ts">
	// Lucide's heart that beats once on `play()`: the caller plays it when a
	// tap sets the mark, so it stays still on arrival and when it empties.
	let { class: className = '' }: { class?: string } = $props();

	let beating = $state(false);

	export function play() {
		beating = true;
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
	class:beating
	onanimationend={() => (beating = false)}
>
	<path
		d="M2 9.5a5.5 5.5 0 0 1 9.591-3.676.56.56 0 0 0 .818 0A5.49 5.49 0 0 1 22 9.5c0 2.29-1.5 4-3 5.5l-5.492 5.313a2 2 0 0 1-3 .019L5 15c-1.5-1.5-3-3.2-3-5.5"
	/>
</svg>

<style>
	svg {
		transform-origin: center;
	}

	/* A double beat, the second one softer, like a pulse. */
	@media (prefers-reduced-motion: no-preference) {
		.beating {
			animation: beat 480ms ease-out;
		}
	}

	@keyframes beat {
		0% {
			scale: 1;
		}
		20% {
			scale: 1.3;
		}
		40% {
			scale: 0.94;
		}
		60% {
			scale: 1.14;
		}
		100% {
			scale: 1;
		}
	}
</style>
