<script lang="ts">
	import { resolve } from '$app/paths';
	import { imageUrl, type RecipeCard as RecipeCardData } from '$lib/api/recipes';
	import { formatMinutes } from '$lib/recipe/format';
	import AuthorInitials from './AuthorInitials.svelte';
	import FavouriteStar from './FavouriteStar.svelte';
	import PlaceholderTile from './PlaceholderTile.svelte';

	let { recipe }: { recipe: RecipeCardData } = $props();

	const time = $derived(formatMinutes(recipe.totalMinutes));
	// `$props.id()` only runs as its own top-level declaration.
	const uid = $props.id();
	const titleId = `${uid}-title`;

	// Handed to the initials' popover as its anchor, so the panel lines up
	// with the card rather than with the two small circles inside it.
	let cardEl = $state<HTMLElement | null>(null);
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

	`<article>` labelled by the title, rather than a bare `<div>`: with the
	anchor no longer wrapping the card, the title is the only thing that
	names it, and this is what gives the whole card back a name - one a
	screen reader can announce and a test can address.
-->
<article
	bind:this={cardEl}
	aria-labelledby={titleId}
	class="group relative overflow-hidden rounded-xl bg-surface shadow-card transition has-[a:focus-visible]:outline-2 has-[a:focus-visible]:outline-offset-2 has-[a:focus-visible]:outline-primary has-[a:hover]:brightness-[0.98] md:rounded-2xl"
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
		<!-- The title is what you recognise the card by, so it wraps rather than
		     cutting off ("Schweineschnitzel mit Bratka…"). Two line heights are
		     reserved either way, so the time pills of a row stay on one line.
		     A phone card leaves 124px of text, which German compounds outrun on
		     their own - so `hyphens-auto` (the document carries the account's
		     locale) lets them break instead of running out of the card unseen.
		     Two lines on a phone - of twelve seeded titles exactly two need a
		     third there, and granting it would raise every row for their sake -
		     but three on desktop, where 19px type pushes three titles past two
		     lines and a four-column grid has the height to spare. -->
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
				<AuthorInitials
					createdByName={recipe.createdByName}
					createdByColor={recipe.createdByColor}
					updatedByName={recipe.updatedByName}
					updatedByColor={recipe.updatedByColor}
					anchor={cardEl}
				/>
			</div>
		</div>
	</div>
	<!-- Offsets are measured from the card's own edge, not the image's: the
	     image sits inset by the card's `p-1.5 md:p-2` padding, so the star
	     needs that padding plus a 6px inset inside the image added back in
	     (top-3/right-3 = 12px = 6px padding + 6px inset; md:top-3.5/right-3.5
	     = 14px = 8px padding + 6px inset) to land inside the image tile with
	     even clearance on both edges, rather than flush on its corner. -->
	<div class="absolute top-3 right-3 md:top-3.5 md:right-3.5">
		<FavouriteStar id={recipe.id} active={recipe.favourite} />
	</div>
</article>
