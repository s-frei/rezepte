<script lang="ts">
	// Lucide's copy icon that confirms a copy in place: the back sheet slides
	// onto the front one, both give way to a check that draws itself, and
	// after a moment the sheets come back. The toast stays the actual message;
	// this is the answer right where the finger is.
	let { class: className = '' }: { class?: string } = $props();

	let copied = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;

	export function play() {
		copied = true;
		clearTimeout(timer);
		timer = setTimeout(() => (copied = false), 1600);
	}

	$effect(() => () => clearTimeout(timer));
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
	class:copied
>
	<g class="sheets">
		<rect width="14" height="14" x="8" y="8" rx="2" ry="2" />
		<path class="back" d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
	</g>
	<!-- Lucide's check, drawn from the short stroke the way a hand writes it. -->
	<path class="check" d="M4 12l5 5L20 6" pathLength="1" />
</svg>

<style>
	.sheets {
		transform-origin: center;
	}

	.check {
		stroke-dasharray: 1;
		stroke-dashoffset: 1;
	}

	.copied .sheets {
		opacity: 0;
		scale: 0.6;
	}

	.copied .back {
		translate: 6px 6px;
	}

	.copied .check {
		stroke-dashoffset: 0;
	}

	/* Without motion the state still changes, it just does not travel. */
	@media (prefers-reduced-motion: no-preference) {
		.sheets {
			transition:
				opacity 160ms ease-in,
				scale 160ms ease-in;
		}

		.back {
			transition: translate 200ms ease-out;
		}

		.check {
			transition: stroke-dashoffset 160ms ease-in;
		}

		.copied .sheets {
			transition-delay: 160ms;
		}

		.copied .check {
			transition: stroke-dashoffset 280ms ease-out 260ms;
		}
	}
</style>
