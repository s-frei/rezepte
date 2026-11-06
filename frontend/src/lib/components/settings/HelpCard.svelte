<script lang="ts">
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import LifeBuoy from '@lucide/svelte/icons/life-buoy';
	import { USER_GUIDE_URL } from '#lib/docs.js';
	import { m } from '#lib/paraglide/messages.js';
</script>

<!--
	The whole card is the link: there is nothing to fill in here, only one place
	to go, so the target is as large as the card instead of a button inside it.
	The circle echoes the avatar at the top of the page, which closes the page
	with the same shape it opens with, and the arrow is the one the API card's
	docs button uses for "leaves the app".

	The guide lives on GitHub Pages, outside the SPA router, so `resolve()` does
	not apply and the navigation rule is lifted for this anchor only.
-->
<!-- eslint-disable svelte/no-navigation-without-resolve -->
<a
	href={USER_GUIDE_URL}
	target="_blank"
	rel="noopener external"
	class="group @container block rounded-2xl bg-surface p-6 transition hover:bg-surface-elevated focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:p-7"
>
	<!-- Below 30rem of card (every phone, and 200% text) the circle and the
	     arrow go above the text, which then has the card's full width:
	     beside them it squeezed to a word per line at 200%. -->
	<span
		class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-4 @max-[30rem]:grid-cols-[minmax(0,1fr)_auto]"
	>
		<span
			aria-hidden="true"
			class="flex size-12 shrink-0 items-center justify-center rounded-full bg-accent text-accent-foreground"
		>
			<LifeBuoy class="size-6" />
		</span>
		<span class="min-w-0 @max-[30rem]:col-span-2">
			<span class="block font-display text-heading font-medium">{m.settings_help_title()}</span>
			<span class="mt-1 block max-w-[60ch] text-caption text-text-muted">
				{m.settings_help_text()}
			</span>
			<span class="sr-only">({m.settings_help_new_tab()})</span>
		</span>
		<ArrowUpRight
			aria-hidden="true"
			class="size-5 shrink-0 text-text-muted transition group-hover:translate-x-0.5 group-hover:-translate-y-0.5 group-hover:text-text motion-reduce:transition-none @max-[30rem]:col-start-2 @max-[30rem]:row-start-1 @max-[30rem]:self-start"
		/>
	</span>
</a>
<!-- eslint-enable svelte/no-navigation-without-resolve -->
