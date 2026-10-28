<script lang="ts">
	let {
		fraction,
		glyph,
		class: className = ''
	}: {
		/** Elapsed share, 0..1: the ring fills clockwise from the top. */
		fraction: number;
		/** What a tap on the ring does: ⏸ while running, ▶ while paused. */
		glyph?: 'pause' | 'play';
		/** Size and color (`currentColor`); the track is the same color, faint. */
		class?: string;
	} = $props();
</script>

<!-- `pathLength` makes the dash a share of 100, whatever the radius. -->
<svg viewBox="0 0 36 36" class="shrink-0 {className}" aria-hidden="true">
	<circle cx="18" cy="18" r="15" fill="none" stroke="currentColor" stroke-width="5" opacity="0.2" />
	<circle
		cx="18"
		cy="18"
		r="15"
		fill="none"
		stroke="currentColor"
		stroke-width="5"
		stroke-linecap="round"
		pathLength="100"
		stroke-dasharray="100"
		stroke-dashoffset={100 * (1 - fraction)}
		transform="rotate(-90 18 18)"
		class="motion-safe:transition-[stroke-dashoffset] motion-safe:duration-1000 motion-safe:ease-linear"
	/>
	{#if glyph === 'pause'}
		<rect x="13.5" y="12.5" width="3" height="11" rx="1" fill="currentColor" />
		<rect x="19.5" y="12.5" width="3" height="11" rx="1" fill="currentColor" />
	{:else if glyph === 'play'}
		<path
			d="M15 12.5v11l9-5.5z"
			fill="currentColor"
			stroke="currentColor"
			stroke-linejoin="round"
		/>
	{/if}
</svg>
