<script lang="ts">
	// Lucide's paper plane that takes off on `play()`: it leaves at the top
	// right and glides back in from the bottom left. The caller plays it when
	// the mail goes out; the result line underneath says how it went.
	let { class: className = '' }: { class?: string } = $props();

	let flying = $state(false);

	export function play() {
		flying = true;
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
	class:flying
	onanimationend={() => (flying = false)}
>
	<g class="plane">
		<path
			d="M14.536 21.686a.5.5 0 0 0 .937-.024l6.5-19a.496.496 0 0 0-.635-.635l-19 6.5a.5.5 0 0 0-.024.937l7.93 3.18a2 2 0 0 1 1.112 1.11z"
		/>
		<path d="m21.854 2.147-10.94 10.939" />
	</g>
</svg>

<style>
	svg {
		overflow: hidden;
	}

	@media (prefers-reduced-motion: no-preference) {
		.flying .plane {
			animation: fly 640ms ease-in-out;
		}
	}

	@keyframes fly {
		0% {
			translate: 0 0;
		}
		45% {
			translate: 24px -24px;
		}
		46% {
			translate: -24px 24px;
		}
		100% {
			translate: 0 0;
		}
	}
</style>
