<script lang="ts" module>
	/** The circle itself, for a menu trigger that cannot be an `IconButton`; the caller adds `bg-*` and `text-*`. */
	export const iconCircle =
		'inline-flex size-10 shrink-0 items-center justify-center rounded-full border border-border hover-tint transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98]';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Button as BitsButton } from 'bits-ui';

	let {
		label,
		href,
		type = 'button',
		disabled = false,
		onclick,
		class: className = '',
		children
	}: {
		label: string;
		/** Makes it a link. */
		href?: string;
		type?: 'button' | 'submit' | 'reset';
		disabled?: boolean;
		onclick?: (event: MouseEvent) => void;
		class?: string;
		children: Snippet;
	} = $props();

	// The default surface steps aside when the caller passes its own bg-* class
	// (no tailwind-merge in this project, and CSS source order would otherwise win).
	const background = $derived(/\bbg-/.test(className) ? '' : 'bg-surface');
	// Likewise the default ink steps aside for a caller's own text-* color.
	const ink = $derived(/\btext-/.test(className) ? '' : 'text-text');
</script>

<BitsButton.Root
	{...href ? { href } : { type }}
	{disabled}
	{onclick}
	aria-label={label}
	class="{iconCircle} {background} {ink} disabled:pointer-events-none disabled:opacity-50 {className}"
>
	{@render children()}
</BitsButton.Root>
