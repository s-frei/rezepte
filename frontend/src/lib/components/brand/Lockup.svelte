<script lang="ts">
	import compact from '$brand/lockups/rezepte-lockup-compact.svg?raw';
	import stacked from '$brand/lockups/rezepte-lockup-stacked.svg?raw';

	// Inlined rather than an <img>, so the ink and sprout groups take the
	// --color-logo-* tokens and follow the app's own theme switch, which a
	// -dark file picked by prefers-color-scheme could not.
	const sources = { compact, stacked };

	let {
		variant,
		label,
		class: className = ''
	}: {
		variant: keyof typeof sources;
		/** Accessible name; leave it out where a surrounding link already names the logo. */
		label?: string;
		class?: string;
	} = $props();
</script>

<span
	class="lockup block {className}"
	role={label ? 'img' : undefined}
	aria-label={label}
	aria-hidden={label ? undefined : 'true'}
>
	<!-- eslint-disable-next-line svelte/no-at-html-tags -- a build-time asset from assets/brand, never user input -->
	{@html sources[variant]}
</span>

<style>
	.lockup :global(svg) {
		display: block;
		height: 100%;
		width: auto;
	}
	.lockup :global(.ink) {
		fill: var(--color-logo-ink);
	}
	.lockup :global(.sprout) {
		fill: var(--color-logo-sprout);
	}
</style>
