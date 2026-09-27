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

	Hover and focus live on this wrapper through `has-[a:…]`, not on the
	anchor, so an effect covers the whole card while still answering to the
	link alone: pointing at the star or the initials leaves the card at rest,
	which is what you want once the hover state grows into an animation.

	`<article>` labeled by the title, rather than a bare `<div>`: with the
	anchor no longer wrapping the card, the title is the only thing that
	names it, and this is what gives the whole card back a name - one a
	screen reader can announce and a test can address.
-->
<article
	bind:this={cardEl}
	aria-labelledby={titleId}
	class="group @container relative overflow-hidden rounded-xl bg-surface shadow-card transition has-[a:focus-visible]:outline-2 has-[a:focus-visible]:outline-offset-2 has-[a:focus-visible]:outline-primary has-[a:hover]:brightness-[0.98] md:rounded-2xl"
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
		<!-- The title is what you recognize the card by, so it wraps rather than
		     cutting off ("Schweineschnitzel mit Bratka…"). Two line heights are
		     reserved either way, so the time pills of a row stay on one line.
		     A phone card leaves 124px of text, which German compounds outrun on
		     their own - so `hyphens-auto` (the document carries the account's
		     locale) lets them break instead of running out of the card unseen.
		     Two lines on a phone - of twelve seeded titles exactly two need a
		     third there, and granting it would raise every row for their sake -
		     but three on desktop, where 19px type pushes three titles past two
		     lines and the desktop grid has the height to spare. -->
		<h3
			id={titleId}
			class="line-clamp-2 min-h-[2lh] font-display text-[16px] font-medium hyphens-auto md:line-clamp-3 md:text-card"
		>
			<!-- The anchor must not be positioned itself, or its `::after` would
			     stretch over the title instead of over the card. -->
			<a
				href={resolve('/recipes/[slug]', { slug: recipe.slug })}
				class="outline-none after:absolute after:inset-0 after:content-['']"
			>
				{recipe.title}
			</a>
		</h3>
		{#if recipe.tags.length > 0}
			<p class="mt-1 truncate text-caption text-text-muted">{recipe.tags.join(', ')}</p>
		{/if}
		<!-- One row for the two pieces of metadata that are not the recipe
		     itself. It renders even when neither the time nor a second
		     author is there, so cards in a row keep the same height. -->
		<div class="mt-2 flex h-5 items-center justify-between gap-2">
			{#if time}
				<span
					class="inline-flex h-5 items-center rounded-pill bg-accent px-2 text-micro font-semibold text-accent-foreground"
				>
					{time}
				</span>
			{:else}
				<span></span>
			{/if}
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
