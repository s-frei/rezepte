<script lang="ts">
	import type { Snippet } from 'svelte';
	import Clock from 'lucide-svelte/icons/clock';
	import ExternalLink from 'lucide-svelte/icons/external-link';
	import { formatMinutes } from '$lib/recipe/format';
	import { m } from '$lib/paraglide/messages';

	let {
		prepMinutes,
		cookMinutes,
		sourceUrl,
		children
	}: {
		prepMinutes: number | null;
		cookMinutes: number | null;
		sourceUrl: string | null;
		/** Rendered first in the row - the detail page puts the servings stepper here. */
		children?: Snippet;
	} = $props();

	const pillClass =
		'inline-flex h-9 items-center gap-1.5 rounded-pill border border-border bg-surface px-3.5 text-body-sm font-medium';
</script>

<div class="flex flex-wrap items-center gap-2.5">
	{#if children}
		{@render children()}
	{/if}
	{#if prepMinutes !== null}
		<span class="{pillClass} text-text">
			<Clock class="size-4" aria-hidden="true" />
			{formatMinutes(prepMinutes)}
			{m.recipe_prep_time()}
		</span>
	{/if}
	{#if cookMinutes !== null}
		<span class="{pillClass} text-text">
			<Clock class="size-4" aria-hidden="true" />
			{formatMinutes(cookMinutes)}
			{m.recipe_cook_time()}
		</span>
	{/if}
	{#if sourceUrl}
		<a href={sourceUrl} target="_blank" rel="noopener external" class="{pillClass} text-primary">
			{m.recipe_source()}
			<ExternalLink class="size-3.5" aria-hidden="true" />
		</a>
	{/if}
</div>
