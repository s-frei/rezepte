<script lang="ts">
	import horizontal from '$brand/lockups/rezepte-lockup-horizontal.svg?url';
	import mark from '$brand/lockups/rezepte-mascot.webp?url';
	import stacked from '$brand/lockups/rezepte-lockup-stacked.svg?url';

	// Images rather than inline SVG: the lockups carry their own cream box and
	// the painted mascot, so they look the same in both themes and nothing in
	// them follows a token.
	const sources = { horizontal, stacked, mark };

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

<img src={sources[variant]} alt={label ?? ''} class="block w-auto {className}" />
