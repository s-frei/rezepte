<script lang="ts">
	import { resolve } from '$app/paths';
	import { imageUrl, type RecipeCard as RecipeCardData } from '$lib/api/recipes';
	import { session } from '$lib/auth.svelte';
	import { formatMinutes } from '$lib/recipe/format';
	import AuthorInitials from './AuthorInitials.svelte';
	import FavoriteStar from './FavoriteStar.svelte';
	import PlaceholderTile from './PlaceholderTile.svelte';
	import TastyButton from './TastyButton.svelte';

	let { recipe }: { recipe: RecipeCardData } = $props();

	const time = $derived(formatMinutes(recipe.totalMinutes));
	// `$props.id()` only runs as its own top-level declaration.
	const uid = $props.id();
	const titleId = `${uid}-title`;

	// Handed to the initials' popover as its anchor, so the panel lines up
	// with the card rather than with the two small circles inside it.
	let cardEl = $state<HTMLElement | null>(null);

	// The service refuses the author's own mark, so the author gets the
	// count without a button to press.
	const own = $derived(session.user?.id === recipe.createdBy.id);
</script>

<!--
	The card is a stretched link, not a link wrapped around the card: the
	anchor holds the title alone and grows an `::after` over the whole card
	for the click target. Everything else - the star, the author initials -
	is then a sibling of the link rather than content inside it, which is
	what lets them be buttons at all (interactive content inside an `<a>` is
	invalid HTML and a focus trap), while Tab still finds one stop per
	interactive thing rather than one per element the card draws.

	Hover and focus live on this wrapper through `has-[a:…]` and the
	`card-hover` variant, not on the anchor, so an effect covers the whole
	card while still answering to the link alone: pointing at the star or
	the initials leaves the card at rest.

	Hovered, the card is picked up like an index card off a pile: it rises,
	tips a little and its shadow deepens, while the `squiggle` line draws
	itself under the title. Every card tips the same way, and arriving and
	leaving are timed apart on purpose - the tip springs in 90ms late, the
	return is quick and flat. With both moving at once and mirrored, running
	the pointer across a row read as one seesaw rocking two cards. Reduced
	motion keeps the shadow and the line and drops the movement.

	`<article>` labeled by the title, rather than a bare `<div>`: with the
	anchor no longer wrapping the card, the title is the only thing that
	names it, and this is what gives the whole card back a name - one a
	screen reader can announce and a test can address.
-->
<article
	bind:this={cardEl}
	aria-labelledby={titleId}
	class="group @container relative origin-bottom overflow-hidden rounded-xl bg-surface shadow-card transition-[translate,rotate,box-shadow] duration-160 ease-out has-[a:focus-visible]:outline-2 has-[a:focus-visible]:outline-offset-2 has-[a:focus-visible]:outline-primary motion-reduce:transition-none md:rounded-2xl card-hover:shadow-lift card-hover:delay-90 card-hover:duration-420 card-hover:ease-spring motion-safe:card-hover:-translate-y-1 motion-safe:card-hover:-rotate-[1.2deg]"
>
	<div class="aspect-square p-1.5 md:p-2">
		{#if recipe.coverImageId}
			<img
				src={imageUrl(recipe.id, recipe.coverImageId, 'thumb')}
				alt=""
				loading="lazy"
				decoding="async"
				class="size-full rounded-lg object-cover"
			/>
		{:else}
			<PlaceholderTile id={recipe.id} title={recipe.title} class="overflow-hidden rounded-lg" />
		{/if}
	</div>
	<div class="px-4 pt-2 pb-[18px]">
		<!-- The time runs above the title like the timing over a cookbook
		     heading, a label rather than a pill: without the pill's padding
		     and with a line of its own, the longest time the service allows
		     ("47 Std 59 Min" - prep and cook are capped at 1440 each) still
		     fits a 320px phone's card, where a pill beside the initials broke
		     onto two lines from "2 Std 45 Min" on. The line is kept when there
		     is no time, so every card of a row starts its title at one height;
		     `truncate` only guards against a catalog that outgrows it. -->
		<p class="mb-1 h-[1lh] truncate text-label font-semibold text-accent-foreground uppercase">
			{time}
		</p>
		<!-- The title is what you recognize the card by, so it wraps rather than
		     cutting off ("Schweineschnitzel mit Bratka…"). Two line heights are
		     reserved either way, so the footer of a row stays on one line.
		     The hover line needs room of its own: `leading-snug` rather than
		     text-card's 1.2 keeps it off the next line's ascenders, and `pb-1`
		     keeps the last line's copy, which hangs below the line box, from
		     being clipped by the clamp.
		     A phone card leaves 124px of text, which German compounds outrun on
		     their own - so `hyphens-auto` (the document carries the account's
		     locale) lets them break instead of running out of the card unseen.
		     Two lines on a phone - of twelve seeded titles exactly two need a
		     third there, and granting it would raise every row for their sake -
		     but three on desktop, where 19px type pushes three titles past two
		     lines and the desktop grid has the height to spare. -->
		<h3
			id={titleId}
			class="line-clamp-2 min-h-[calc(2lh+4px)] pb-1 font-display text-card-sm leading-snug font-medium hyphens-auto md:line-clamp-3 md:text-card"
		>
			<!-- The anchor must not be positioned itself, or its `::after` would
			     stretch over the title instead of over the card. -->
			<a
				href={resolve('/recipes/[slug]', { slug: recipe.slug })}
				class="outline-none after:absolute after:inset-0 after:content-['']"
			>
				<span class="squiggle">{recipe.title}</span>
			</a>
		</h3>
		<!-- One footer for the tags and the authors. It renders whatever is
		     missing, so the initials of a row sit at one height; the tags give
		     way with an ellipsis, the initials never do. -->
		<div class="mt-2 flex h-5 items-center justify-between gap-2">
			<p class="min-w-0 truncate text-caption text-text-muted">{recipe.tags.join(', ')}</p>
			<!-- `relative` lifts the initials out of the link's `::after`, so a
			     tap reaches their popover instead of opening the recipe. -->
			<div class="relative">
				<AuthorInitials createdBy={recipe.createdBy} updatedBy={recipe.updatedBy} anchor={cardEl} />
			</div>
		</div>
	</div>
	<!-- Offsets are measured from the card's own edge, not the image's: the
	     image sits inset by the card's `p-1.5 md:p-2` padding, so the star
	     needs that padding plus a 6px inset inside the image added back in
	     (md:top-3.5/right-3.5 = 14px = 8px padding + 6px inset) to land
	     inside the image tile with even clearance on both edges, rather than
	     flush on its corner.

	     Only from md up. On a phone the image is about 150px wide and a 36px
	     circle covers too much of it, so the star is set on the recipe page
	     there - the card keeps the photo. -->
	<div class="absolute top-3.5 right-3.5 hidden md:block">
		<FavoriteStar id={recipe.id} active={recipe.favorite} />
	</div>
	<!-- The tasty pill mirrors the star into the image's bottom-left corner.
	     The image is square and fills the card's width, so its bottom edge is
	     the card's width below the top: `top` is that width (100cqw, the card
	     is the size container) minus the padding and the same 6px inset -
	     6 + 6 = 12px on a phone, 8 + 6 = 14px from md up - and the pill hangs
	     above that line (`-translate-y-full`), whatever height it has. `left`
	     is padding plus inset, as for the star.

	     From md up it is a button, painted over the link's `::after` because
	     it comes later in the document. On a phone, and on the author's own
	     recipe everywhere, it is the count alone, and only once somebody
	     marked the recipe. On a phone that count is `fluid`: it grows with
	     the card, not with the viewport, so a wide phone card never shows a
	     smaller heart than the narrower desktop card just past md.
	     `pointer-events-none` lets a tap fall through to
	     the link, since marking happens on the recipe page and the author
	     cannot mark their own. -->
	<div
		class="pointer-events-none absolute top-[calc(100cqw_-_12px)] left-3 -translate-y-full md:top-[calc(100cqw_-_14px)] md:left-3.5 {own
			? ''
			: 'md:pointer-events-auto'}"
	>
		{#if recipe.tastyCount > 0}
			<span class="md:hidden">
				<TastyButton id={recipe.id} count={recipe.tastyCount} readonly fluid />
			</span>
		{/if}
		{#if !own || recipe.tastyCount > 0}
			<span class="hidden md:inline">
				<TastyButton
					id={recipe.id}
					active={recipe.tasty}
					count={recipe.tastyCount}
					readonly={own}
				/>
			</span>
		{/if}
	</div>
</article>
