<script lang="ts">
	import type { Snippet } from 'svelte';
	import kitchen from '$brand/lockups/rezepte-kitchen.webp?url';
	import wordmark from '$brand/lockups/rezepte-wordmark.svg?url';
	import { m } from '$lib/paraglide/messages';

	let {
		lead = '',
		children
	}: {
		/** The line under the wordmark; left out when empty. */
		lead?: string;
		/** The card's contents. */
		children: Snippet;
	} = $props();
</script>

<!-- Two layouts from one tree. From lg the kitchen scene covers the window and
     the card lies on its empty right third. Below lg the page opens like a
     cookbook: the scene as a plate on top, the fold's shadow, then a text
     page with the wordmark, a printer's ornament and the card's contents. -->
<main class="relative flex min-h-screen flex-col bg-surface lg:block lg:bg-transparent">
	<img
		src={kitchen}
		alt=""
		class="h-[250px] w-full object-cover object-[25%_100%] sm:h-[320px] lg:absolute lg:inset-0 lg:h-full lg:object-[52%_100%]"
	/>
	<div
		class="relative flex flex-1 flex-col items-center gap-3.5 px-6 py-8 before:absolute before:inset-x-0 before:top-0 before:h-[22px] before:bg-linear-to-b before:from-overlay/50 before:to-transparent lg:ml-auto lg:min-h-screen lg:w-[460px] lg:items-stretch lg:justify-center lg:gap-5 lg:px-12 lg:py-14 lg:before:hidden dark:before:from-lightbox/70"
	>
		<!-- The wordmark as a mask, so on the text page it can take the theme's
		     ink; on the painting it keeps its own. Quoted: Vite inlines the small
		     SVG as a data URI, whose parentheses break a bare url(). -->
		<span
			role="img"
			aria-label={m.app_name()}
			style:mask-image={`url("${wordmark}")`}
			class="block aspect-[386.8/94.6] w-[200px] bg-accent-foreground mask-contain mask-center mask-no-repeat forced-color-adjust-none lg:w-[250px] lg:self-center lg:bg-scene-ink"
		></span>
		<div
			aria-hidden="true"
			class="flex w-[180px] items-center gap-3 text-body text-primary before:h-px before:flex-1 before:bg-border after:h-px after:flex-1 after:bg-border lg:hidden"
		>
			❦
		</div>
		{#if lead}
			<p
				class="mb-1.5 text-center font-display text-body-lg text-balance text-text-muted italic lg:-mt-2.5 lg:mb-0 lg:font-sans lg:text-scene-muted lg:not-italic"
			>
				{lead}
			</p>
		{/if}
		<!-- empty:hidden: a page still loading passes nothing yet, and an empty
		     card would flash on the painting. -->
		<div
			class="flex w-full max-w-[440px] flex-col gap-5 empty:hidden lg:rounded-2xl lg:bg-surface lg:p-6 lg:shadow-lift dark:lg:border dark:lg:border-border"
		>
			{@render children()}
		</div>
	</div>
</main>
