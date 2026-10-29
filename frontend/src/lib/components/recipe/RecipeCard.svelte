<script lang="ts">
	import { resolve } from '$app/paths';
	import { imageUrl, type RecipeCard as RecipeCardData } from '$lib/api/recipes';
	import { session } from '$lib/auth.svelte';
	import { formatMinutes } from '$lib/recipe/format';
	import { initialFor, tintFor } from '$lib/recipe/placeholder';
	import AuthorInitials from './AuthorInitials.svelte';
	import FavoriteStar from './FavoriteStar.svelte';
	import TastyButton from './TastyButton.svelte';

	let {
		recipe,
		rise = null
	}: {
		recipe: RecipeCardData;
		/** Set on a card "Load more" brought: its place in that batch, which
		 * staggers its rise into the list. */
		rise?: number | null;
	} = $props();

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

	// A cover that fails to load turns the card into a text card rather than
	// leaving a broken picture. Keyed by the image id, so a new cover gets its
	// own chance without an effect resetting a flag.
	let failed = $state<string | null>(null);
	const photo = $derived(recipe.coverImageId !== null && failed !== recipe.coverImageId);
	// The photo fades in once it has loaded, over the skeleton's tint that
	// holds its place, rather than popping in as the list scrolls. One the
	// browser already has (a way back from the recipe) is there at once and
	// never fades. Keyed by the image id, as `failed` is.
	let loaded = $state<string | null>(null);
	function seeCached(img: HTMLImageElement) {
		if (img.complete && img.naturalWidth > 0) {
			loaded = recipe.coverImageId;
		}
	}

	// The photo keeps its own shape, between 4:5 portrait and 16:9 landscape;
	// `object-cover` crops what lies beyond. The size comes with the card, so
	// the box has its height before the picture loads and the grid places
	// the card once. Square when the service does not know the size.
	const ratio = $derived(
		recipe.coverWidth && recipe.coverHeight
			? Math.min(Math.max(recipe.coverWidth / recipe.coverHeight, 4 / 5), 16 / 9)
			: 1
	);

	// The title's type, shared by the title and the hover line's layer over
	// it, so both break their lines at the same places.
	const titleType = $derived(
		`line-clamp-3 font-display font-medium hyphens-auto [hyphenate-limit-chars:6_3_3] [grid-area:1/1] ${
			photo
				? 'text-card-sm leading-snug desk:text-card'
				: 'pt-0.5 pr-8 text-card-cover wrap-anywhere desk:pr-9'
		}`
	);
</script>

<!--
	The tasty count from md up: a button, unless the recipe is the viewer's
	own, where it is the count alone. Only the button is lifted over the
	link's `::after`; a tap on the count falls through to the link, since the
	author cannot mark their own. On a photo it is the `card` button, on a
	text card the `compact` pill, as tall as the initials beside it with
	their ring. A phone shows the count alone, in the footer (see below).
-->
{#snippet tasty(compact: boolean)}
	{#if !own || recipe.tastyCount > 0}
		<span class="hidden desk:inline-flex {own ? '' : 'pointer-events-auto relative'}">
			<TastyButton
				id={recipe.id}
				active={recipe.tasty}
				count={recipe.tastyCount}
				readonly={own}
				size={compact ? 'compact' : 'card'}
			/>
		</span>
	{/if}
{/snippet}

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

	Corners are concentric: a round mark in a rounded corner shares the
	corner's center, so corner radius = mark radius + gap. The card is 28px
	(24 on a phone), the photo 20 (18) inside an 8px (6) inset; the star's
	14px offset puts its 14px radius's center on the photo corner's center
	(20 from each edge), and the text card's 28 = 14 + 14 follows the same
	rule. The initials share the bottom-right corner's center the same way,
	which puts them on the star's axis, and the tasty pill on a photo the
	photo's bottom-left one.

	A card "Load more" brought rises into place from a little below,
	staggered through its batch (50ms each, the ninth on together), so the
	new cards read as arriving rather than as the page jumping. The place in
	the batch is `--rise`, so the overview's arrival after a sign-in
	(`data-arrival`, app.css) can retime the same rise. Only the
	`transform` animates: placement is `left`/`top` and the hover tilt uses
	`translate` and `rotate`, which compose with it. Reduced motion drops it.

	Two shapes, each as tall as its content, since the grid is a masonry:
	a photo card, the photo above the text, and a text card for a recipe
	without one, filled with the recipe's tint. `isolate` makes the card the
	stacking context the text card's initial hides behind its text in.
-->
<article
	bind:this={cardEl}
	data-card={recipe.id}
	aria-labelledby={titleId}
	class="group @container relative isolate origin-bottom overflow-hidden rounded-3xl shadow-card transition-[translate,rotate,box-shadow] duration-160 ease-out has-[a:focus-visible]:outline-2 has-[a:focus-visible]:outline-offset-2 has-[a:focus-visible]:outline-primary motion-reduce:transition-none card-hover:shadow-lift card-hover:delay-90 card-hover:duration-420 card-hover:ease-spring motion-safe:card-hover:-translate-y-1 motion-safe:card-hover:-rotate-[1.2deg] desk:rounded-4xl {rise ===
	null
		? ''
		: '[animation-delay:calc(var(--rise)*50ms)] motion-safe:animate-rise'} {photo
		? 'bg-surface'
		: tintFor(recipe.id)}"
	style:--rise={rise === null ? undefined : Math.min(rise, 8)}
>
	{#if photo}
		<!-- The photo sits inset from the card's edge on three sides; the
		     text below is its fourth. -->
		<div class="relative px-1.5 pt-1.5 desk:px-2 desk:pt-2">
			<div class="rounded-xl bg-skeleton desk:rounded-2xl">
				<img
					src={imageUrl(recipe.id, recipe.coverImageId!, 'thumb')}
					alt=""
					data-cover={recipe.slug}
					loading="lazy"
					decoding="async"
					{@attach seeCached}
					onload={() => (loaded = recipe.coverImageId)}
					onerror={() => (failed = recipe.coverImageId)}
					style:aspect-ratio={ratio}
					class="block w-full rounded-xl object-cover transition-opacity duration-200 motion-reduce:transition-none desk:rounded-2xl {loaded ===
					recipe.coverImageId
						? ''
						: 'opacity-0'}"
				/>
			</div>
			<!-- From md up the button mirrors the star into the photo's
			     bottom-left corner, 6px inside it: the photo box ends where the
			     photo does, so `bottom` is the inset alone and `left` is the
			     box's padding plus it. A flex box, so no line box puts a strut
			     under the pill. The link's `::after` comes later in the
			     document, so `z-10` paints the button over it, and
			     `pointer-events-none` hands taps on the count alone back to the
			     link. -->
			<div class="pointer-events-none absolute bottom-1.5 left-3.5 z-10 flex">
				{@render tasty(false)}
			</div>
		</div>
	{:else}
		<!-- The recipe's initial, large and faint, at the card's right edge
		     and centered on it, so it bleeds past the top and bottom and the
		     card's `overflow-hidden` crops it. `-z-10` lays it under the text,
		     the star and the initials. -->
		<span
			aria-hidden="true"
			class="pointer-events-none absolute top-1/2 right-2.5 -z-10 -translate-y-1/2 font-display text-initial-sm leading-none font-medium text-primary italic opacity-14 desk:right-[18px] desk:text-initial"
		>
			{initialFor(recipe.title)}
		</span>
	{/if}
	<!-- One inner padding for the text of both shapes, 12px on a phone and
	     16px from md up; the text card starts lower, where it has no photo
	     above it. Not positioned, or it would catch the link's `::after`
	     instead of the card. -->
	<div
		class={photo
			? 'px-3 pt-2 pb-3 desk:px-4 desk:pt-2.5 desk:pb-4'
			: 'px-3 pt-3.5 pb-3 desk:px-4 desk:pt-5 desk:pb-4'}
	>
		<!-- The time runs above the title like the timing over a cookbook
		     heading, a label rather than a pill: without the pill's padding
		     and with a line of its own, the longest time the service allows
		     ("47 Std 59 Min" - prep and cook are capped at 1440 each) still
		     fits a 320px phone's card. Without a time there is no line. -->
		{#if time}
			<!-- The text card's gap is wider: its display title has more
			     internal leading (1.15 at up to 30px) than the photo card's 1.2
			     at 16-19px, and the label would otherwise look detached from
			     the glyphs rather than sit on them. -->
			<p
				class="truncate text-label font-semibold text-accent-foreground uppercase {photo
					? 'mb-0.5'
					: 'mb-1.5'}"
			>
				{time}
			</p>
		{/if}
		<!-- The title is what you recognize the card by, so it wraps, up to
		     three lines. A phone card leaves 124px of text, which German
		     compounds outrun on their own - so `hyphens-auto` (the document
		     carries the account's locale) lets them break instead of running
		     out of the card unseen, never with fewer than three letters on
		     either side. On a text card the title is the cover: the display
		     face in primary, sized to the card (16px on a 320px phone, 30px on
		     desktop), clear of the star from md up, and `wrap-anywhere` breaks
		     a word with no hyphenation point rather than cut it off.

		     The clamp cuts below the third line, and what it lets through is
		     a narrow band: the display face's descenders reach below the line
		     box and a fourth line's accents (Ä, Ö, Ü) reach above it. 0.08em
		     of padding keeps the descenders and still clips the accents at
		     every size the cards use - with the text card's leading at 1.25;
		     at 1.15 the two overlap on a phone and no padding separates them.
		     The hover line hangs lower than that band, so it lives on a layer
		     of its own in the same grid cell: the title again, transparent
		     and hidden from assistive tech, whose clamp leaves 0.35em for the
		     wave and shows nothing else, since its fourth line is invisible.
		     One `minmax(0, 1fr)` column gives both exactly the same width, so
		     a line never breaks in one and not the other. The negative
		     margins give the padding back to the footer. -->
		<div class="grid grid-cols-1">
			<h3
				id={titleId}
				class="-mb-[0.08em] pb-[0.08em] {titleType} {photo
					? ''
					: 'text-accent-foreground desk:text-primary'}"
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
			<p
				aria-hidden="true"
				class="pointer-events-none -mb-[0.35em] pb-[0.35em] text-transparent select-none {titleType}"
			>
				<span class="squiggle">{recipe.title}</span>
			</p>
		</div>
		<!-- From md up the tags sit left of the marks and give way: two lines
		     at most, ending in an ellipsis. A phone card has no room beside
		     its marks, so the tags take a line of their own, two on a photo
		     card and one on a text card, whose title already fills it, and
		     the tasty count and the initials share the line below, one in
		     each bottom corner. -->
		<div
			class="flex flex-wrap items-center gap-2 gap-y-1.5 desk:flex-nowrap {photo
				? 'mt-2 desk:mt-2.5'
				: 'mt-3.5 desk:mt-[18px]'}"
		>
			{#if recipe.tags.length > 0}
				<p
					class="min-w-0 basis-full text-caption desk:line-clamp-2 desk:basis-auto desk:whitespace-normal {photo
						? 'line-clamp-2 text-text-muted'
						: 'truncate text-text'}"
				>
					{recipe.tags.join(', ')}
				</p>
			{/if}
			<!-- The count mirrors the initials into the bottom-left corner,
			     its end cap's center on the corner's center, by the same
			     negative margin. In the accent's sand on a photo card, whose
			     light surface would swallow a light pill in dark mode; light on
			     the tinted text card. Not positioned, so a tap falls through to
			     the link. -->
			{#if recipe.tastyCount > 0}
				<span class="-mb-0.5 -ml-0.5 flex self-end desk:hidden">
					<TastyButton
						id={recipe.id}
						count={recipe.tastyCount}
						readonly
						size="compact"
						accent={photo}
					/>
				</span>
			{/if}
			<!-- The initials share the star's axis: their center is the center
			     of the card's bottom-right corner, as the star's is of the
			     top-right one (28px from both edges, 24 on a phone). The
			     wrapper's 2px padding holds the circles' ring, so the group is
			     as tall as the pill beside it on every card; the negative
			     margin moves it from the text's 16px (12) inset to the corner's
			     14px (12) for the circle, less the ring. -->
			<div
				class="-mr-0.5 -mb-0.5 ml-auto flex shrink-0 items-center gap-1.5 self-end desk:-mr-1 desk:-mb-1"
			>
				{#if !photo}
					{@render tasty(true)}
				{/if}
				<!-- `relative` lifts the initials out of the link's `::after`,
				     so a tap reaches their popover instead of opening the
				     recipe. -->
				<div class="relative p-0.5">
					<AuthorInitials
						createdBy={recipe.createdBy}
						updatedBy={recipe.updatedBy}
						anchor={cardEl}
						fresh={recipe.newComments}
					/>
				</div>
			</div>
		</div>
	</div>
	<!-- Offsets are measured from the card's own edge: on a photo card the
	     photo sits inset by the card's 8px padding, so the star needs that
	     padding plus a 6px inset inside the photo (top-3.5/right-3.5 = 14px)
	     to land inside it with even clearance on both edges, rather than
	     flush on its corner. A text card keeps it at the same place.

	     On a phone the photo's corner is 18px at a 6px inset, so the 24px
	     disc sits 12px from the card's edge; the text card's 28px disc sits
	     10px in, its center on the 24px corner's. -->
	<div class="absolute {photo ? 'top-3 right-3' : 'top-2.5 right-2.5'} desk:top-3.5 desk:right-3.5">
		<FavoriteStar id={recipe.id} active={recipe.favorite} card {photo} />
	</div>
</article>
