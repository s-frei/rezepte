<script lang="ts">
	import wordmark from '$brand/lockups/rezepte-wordmark.svg?url';
	import Lockup from './Lockup.svelte';

	let {
		label,
		href,
		wordmarkClass = ''
	}: {
		/** The tab's accessible name, the app's name. */
		label: string;
		/** Where the tab leads; without it the tab only names the app. */
		href?: string;
		/** Extra classes for the wordmark, e.g. to hide it where the bar is tight. */
		wordmarkClass?: string;
	} = $props();

	const tab =
		'flex shrink-0 items-center gap-2 self-start rounded-b-2xl border border-t-0 border-border bg-surface pt-3.5 pr-3 pb-2.5 pl-3 shadow-card lg:pr-4.5';
</script>

<!--
	A tab hanging from the window's top edge, like the book's ribbon: card
	stock in both themes, where the lockup's cream box was the one light block
	on a dark page. The top bar and the public recipe page share it, so a
	shared link opens on the same mark the app wears.
-->
{#snippet content()}
	<Lockup variant="mark" class="h-10" />
	<!-- Quoted: Vite inlines the small SVG as a data URI, whose parentheses
	     break a bare url(). Nudged down: the "p" descender makes the centered
	     box read high. -->
	<span
		aria-hidden="true"
		style:mask-image={`url("${wordmark}")`}
		class="block aspect-[386.8/94.6] h-6 translate-y-0.5 bg-accent-foreground mask-contain mask-center mask-no-repeat forced-color-adjust-none {wordmarkClass}"
	></span>
{/snippet}

{#if href}
	<a
		{href}
		aria-label={label}
		class="{tab} focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
	>
		{@render content()}
	</a>
{:else}
	<span role="img" aria-label={label} class={tab}>
		{@render content()}
	</span>
{/if}
