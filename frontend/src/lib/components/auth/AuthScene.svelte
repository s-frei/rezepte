<script lang="ts">
	import type { Snippet } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import kitchen from '$brand/lockups/rezepte-kitchen.webp?url';
	import wordmark from '$brand/lockups/rezepte-wordmark.svg?url';
	import { m } from '#lib/paraglide/messages.js';

	let {
		lead = '',
		children
	}: {
		/** The line under the wordmark; left out when empty. */
		lead?: string;
		/** The card's contents. */
		children: Snippet;
	} = $props();

	// Nothing moves as the page opens: the field focused on arrival, a value
	// filled in from the last page and the card lifted for it are simply
	// there. The first press of a key or a pointer lets the card and its
	// fields answer from then on (`data-ready`).
	let ready = $state(false);
	const arm = () => (ready = true);

	let card: HTMLElement;

	/** Shakes the card, the answer to a sign-in the service refused. */
	export function shake() {
		if (prefersReducedMotion.current) {
			return;
		}
		// `transform`, not `translate`: the lift while a field has focus is a
		// `translate`, and the two compose.
		card.animate(
			{ transform: [0, -9, 8, -5, 3, -1, 0].map((x) => `translateX(${x}px)`) },
			{ duration: 460, easing: 'cubic-bezier(0.36, 0.07, 0.19, 0.97)' }
		);
	}
</script>

<svelte:window onkeydowncapture={arm} onpointerdowncapture={arm} />

<!-- Two layouts from one tree. From lg the kitchen scene covers the window and
     the card lies on its empty right third. Below lg the page opens like a
     cookbook: the scene as a plate on top, the fold's shadow, then a text
     page with the wordmark, a printer's ornament and the card's contents. -->
<main
	data-ready={ready || undefined}
	class="relative flex min-h-screen flex-col bg-surface lg:block lg:bg-transparent"
>
	<img
		src={kitchen}
		alt=""
		class="h-[250px] w-full object-cover object-[25%_100%] sm:h-[320px] lg:absolute lg:inset-0 lg:h-full lg:object-[52%_100%]"
	/>
	<!-- `data-auth-page` and `data-auth-card`: what grows into the app after a
	     sign-in (`signIn` in #lib/page-transition.ts), the text page below lg
	     and the card from lg. -->
	<div
		data-auth-page
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
		     card would flash on the painting. While a field has focus the card
		     is picked up off the table: it lifts on a soft spring and its
		     shadow deepens. -->
		<div
			bind:this={card}
			data-auth-card
			class="flex w-full max-w-[440px] flex-col gap-5 empty:hidden in-data-ready:transition-[translate,box-shadow] in-data-ready:duration-520 in-data-ready:ease-spring lg:rounded-2xl lg:bg-surface lg:p-6 lg:shadow-lift lg:focus-within:shadow-dialog motion-safe:lg:focus-within:-translate-y-1.25 dark:lg:border dark:lg:border-border"
		>
			{@render children()}
		</div>
	</div>
</main>
