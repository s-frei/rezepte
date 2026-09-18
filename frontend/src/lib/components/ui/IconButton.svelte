<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Button as BitsButton } from 'bits-ui';

	let {
		label,
		type = 'button',
		disabled = false,
		onclick,
		class: className = '',
		children
	}: {
		label: string;
		type?: 'button' | 'submit' | 'reset';
		disabled?: boolean;
		onclick?: (event: MouseEvent) => void;
		class?: string;
		children: Snippet;
	} = $props();

	// The default surface steps aside when the caller passes its own bg-* class
	// (no tailwind-merge in this project, and CSS source order would otherwise win).
	const background = $derived(/\bbg-/.test(className) ? '' : 'bg-surface');
</script>

<BitsButton.Root
	{type}
	{disabled}
	{onclick}
	aria-label={label}
	class="inline-flex size-10 items-center justify-center rounded-full border border-border {background} text-text transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] disabled:pointer-events-none disabled:opacity-50 {className}"
>
	{@render children()}
</BitsButton.Root>
