<script lang="ts">
	import ExternalLink from '@lucide/svelte/icons/external-link';
	import { m } from '$lib/paraglide/messages';
	import { sourceLabel } from '$lib/recipe/source';

	let { name, url }: { name: string | null; url: string | null } = $props();

	const label = $derived(sourceLabel(name, url));
</script>

<!--
	The credit a cookbook prints under a recipe's title. It exists only when
	there is something to credit, so "Adapted from" never stands alone, and it
	wraps rather than truncates: a cookbook's name with its page number is the
	case this line is for.
-->
{#if label}
	<p class="font-display text-body text-text-muted italic">
		{m.recipe_source_prefix()}
		{#if url}
			<a
				href={url}
				target="_blank"
				rel="noopener external"
				class="text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>{label}<ExternalLink class="ml-1 inline size-3.5 align-[-0.125em]" aria-hidden="true" /></a
			>
		{:else}
			{label}
		{/if}
	</p>
{/if}
