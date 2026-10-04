<script lang="ts">
	import type { Snippet } from 'svelte';

	// A rubber stamp across a tag: what happened to a token or a link, in the
	// one place of the list allowed to be loud. Decoration only - the caller
	// says the same in words that assistive technology reads.
	let {
		tone = 'destructive',
		class: extra = '',
		children
	}: {
		/** `destructive` for gone or paused, `caution` for held back but reversible. */
		tone?: 'destructive' | 'caution';
		/** Placement only. */
		class?: string;
		children: Snippet;
	} = $props();
</script>

<span
	aria-hidden="true"
	class="pointer-events-none inline-block -rotate-6 rounded-md border-[3px] border-double bg-surface-elevated/60 px-2.5 pt-0.5 text-center font-display text-body-sm leading-tight font-bold opacity-85 md:px-3 md:text-card-sm {tone ===
	'caution'
		? 'border-accent-foreground text-accent-foreground'
		: 'border-destructive text-destructive'} {extra}"
>
	{@render children()}
</span>
