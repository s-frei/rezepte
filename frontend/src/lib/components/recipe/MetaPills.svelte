<script lang="ts">
	import Clock from 'lucide-svelte/icons/clock';
	import ExternalLink from 'lucide-svelte/icons/external-link';
	import { formatMinutes } from '$lib/recipe/format';
	import { m } from '$lib/paraglide/messages';

	let {
		servings,
		prepMinutes,
		cookMinutes,
		sourceUrl
	}: {
		servings: number;
		prepMinutes: number | null;
		cookMinutes: number | null;
		sourceUrl: string | null;
	} = $props();

	const pillClass =
		'inline-flex h-9 items-center gap-1.5 rounded-pill border border-border bg-surface px-3.5 text-body-sm font-medium';
</script>

<div class="flex flex-wrap items-center gap-2.5">
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
	<span class="{pillClass} text-text">{servings} {m.recipe_servings()}</span>
	{#if sourceUrl}
		<a href={sourceUrl} target="_blank" rel="noopener external" class="{pillClass} text-primary">
			{m.recipe_source()}
			<ExternalLink class="size-3.5" aria-hidden="true" />
		</a>
	{/if}
</div>
