<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title,
		text,
		children
	}: {
		title: string;
		text: string;
		children: Snippet;
	} = $props();
</script>

<div class="flex flex-col items-center gap-4 px-6 py-16 text-center">
	<div
		aria-hidden="true"
		class="flex size-[120px] items-center justify-center rounded-full bg-accent italic-centered initial-centered font-display text-[64px] font-medium text-primary italic"
	>
		R
	</div>
	<h2 class="font-display text-[26px] font-medium">{title}</h2>
	<p class="max-w-sm text-body text-text-muted">{text}</p>
	<div class="mt-2 flex flex-wrap items-center justify-center gap-3">
		{@render children()}
	</div>
</div>
