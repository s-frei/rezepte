<script lang="ts">
	import X from '@lucide/svelte/icons/x';
	import type { Author } from '#lib/api/recipes.js';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { userColorClasses } from '#lib/user/color.js';

	let {
		person,
		self,
		countShown,
		onclear
	}: {
		/** The one author the grid is narrowed to. */
		person: Author;
		/** The viewer: "Your recipes". */
		self: boolean;
		/** From md, the count only when the running head's "N found" says another number. */
		countShown: boolean;
		/** Drops the author filter - the phone's ×, where the band stands in for the chip. */
		onclear: () => void;
	} = $props();

	const title = $derived(
		self ? m.overview_author_own() : m.person_card_recipes_by({ name: person.displayName })
	);
</script>

<!--
	A small ProfileHead over the grid: the same flat band in the person's
	color with their initial set into it, the photo on its lower edge ringed in
	the page's ground. Only the band is colored; the grid stays as it is.
	Phones: the name alone (the screen reader still hears "Recipes by …"), and
	the band's × replaces the person's filter chip. Desktop keeps the chip and
	its ×, so the band carries none: one way to drop a filter per screen.
-->
<!--
	The text is in the flow, so the band grows with it: 200% text on a phone
	wraps the name instead of cutting it to "M.". The band is a container, and
	below 17rem of it (large text on a phone, where the photo's and the ×'s
	insets left 60px) the text takes the full width over the photo, whose top
	half the band's padding keeps clear.
-->
<div
	class="@container relative mb-10 flex min-h-16 items-center rounded-2xl md:mb-11 md:min-h-18 {userColorClasses(
		person.color
	)}"
>
	<div aria-hidden="true" class="pointer-events-none absolute inset-0 overflow-hidden rounded-2xl">
		<!-- From md the band is 72px and the initial 150: it shows the glyph's
		     foot, which tells a J from a T, rather than its top. -->
		<span
			class="absolute -top-3 -right-1.5 font-display text-initial-sm leading-none font-medium italic opacity-10 md:-top-14 md:right-6 md:text-initial md:opacity-14"
		>
			{person.displayName.charAt(0).toUpperCase()}
		</span>
	</div>
	<span class="absolute -bottom-5.5 left-3.5 md:-bottom-6.5 md:left-5.5">
		<PersonMark {person} size="band" class="ring-4 ring-background" />
	</span>
	<div
		class="relative flex min-w-0 flex-1 flex-col gap-px py-2.5 pr-2 pl-19.5 md:flex-row md:flex-wrap md:items-baseline md:gap-x-3 md:pt-7 md:pr-6 md:pb-2 md:pl-24.5 @max-[17rem]:pt-3 @max-[17rem]:pb-[min(2rem,32px)] @max-[17rem]:pl-3.5"
	>
		<h2
			class="min-w-0 font-display text-card leading-tight font-medium tracking-tight wrap-break-word hyphens-auto md:text-heading"
		>
			{#if self}
				{title}
			{:else}
				<span aria-hidden="true" class="md:hidden">{person.displayName}</span><span
					class="max-md:sr-only">{title}</span
				>
			{/if}
		</h2>
		<span class="text-caption font-semibold opacity-85 {countShown ? '' : 'md:hidden'}">
			{person.count === 1
				? m.person_card_recipes_one()
				: m.person_card_recipes({ count: person.count })}
		</span>
	</div>
	<button
		type="button"
		onclick={onclear}
		aria-label={m.overview_filter_remove({ name: person.displayName })}
		class="relative mr-2.5 flex size-9 shrink-0 items-center justify-center self-center rounded-full ring-1 ring-current/30 ring-inset after:absolute after:-inset-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:hidden @max-[17rem]:mt-2.5 @max-[17rem]:self-start"
	>
		<X class="size-4" aria-hidden="true" />
	</button>
</div>
