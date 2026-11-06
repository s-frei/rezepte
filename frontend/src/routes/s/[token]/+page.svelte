<script lang="ts">
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import Moon from '@lucide/svelte/icons/moon';
	import Sun from '@lucide/svelte/icons/sun';
	import { MediaQuery } from 'svelte/reactivity';
	import { publicImageUrl } from '#lib/api/public.js';
	import { PROJECT_URL, SOURCE_URL } from '#lib/docs.js';
	import type { ImageVariant, RecipeContent } from '#lib/api/recipes.js';
	import BrandTab from '#lib/components/brand/BrandTab.svelte';
	import Lockup from '#lib/components/brand/Lockup.svelte';
	import RecipeView from '#lib/components/recipe/RecipeView.svelte';
	import IconButton from '#lib/components/ui/IconButton.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import Lightbox from '#lib/components/ui/Lightbox.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { setTheme, theme } from '#lib/theme.svelte.js';
	import { userColorClasses } from '#lib/user/color.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// `RecipeContent` from the public response plus a client-side-only id
	// (servings memory, gallery keys - `RecipeView` never sends it to the
	// server). The real recipe id is never in this page's reach: the
	// backend's PublicRecipe carries no id at all. Its images carry no
	// `position` either (nothing here reorders them) - the array is already
	// in gallery order, so the index stands in for it.
	const recipe = $derived<RecipeContent | null>(
		data.recipe && {
			...data.recipe,
			id: `share:${data.token}`,
			images: data.recipe.images.map((image, position) => ({ ...image, position }))
		}
	);

	const sharedBy = $derived(data.recipe?.sharedBy);
	// Translated whole, then split where the name goes, so each language keeps
	// its own order and the name alone is set bold (as RecipeColophon does).
	const WHO = '\u0000';
	const sharedByParts = $derived(
		m
			.public_shared_by({ name: WHO })
			.split(WHO)
			.flatMap((part, i) => (i > 0 ? [WHO, part] : [part]))
			.filter(Boolean)
	);

	function imageSrc(imageId: string, variant: ImageVariant): string {
		return publicImageUrl(data.token, imageId, variant);
	}

	// A stranger gets two states, not the app's three: the button flips what
	// is on screen, and "system" is only where a first visit starts. The
	// choice lands in the same storage as the app's, so an owner opening
	// their own link keeps it.
	const systemDark = new MediaQuery('(prefers-color-scheme: dark)');
	const dark = $derived(theme.value === 'dark' || (theme.value === 'system' && systemDark.current));

	let lightboxOpen = $state(false);
	let lightboxIndex = $state(0);

	function openLightbox(index: number) {
		lightboxIndex = index;
		lightboxOpen = true;
	}
</script>

<svelte:head>
	<title>{recipe ? `${recipe.title} · ${m.app_name()}` : m.app_name()}</title>
</svelte:head>

<div class="mx-auto flex min-h-dvh max-w-[1280px] flex-col px-5 pb-8 md:px-8 md:pb-10">
	<!-- The app's own tab hangs from the top edge here too, so a shared link
	     opens on the mark the app wears; not a link, since the app behind it
	     is the household's, not the visitor's. -->
	<header class="mb-6 flex h-16 items-center justify-between gap-4 md:mb-8">
		<BrandTab label={m.app_name()} />
		<IconButton
			label={dark ? m.public_share_theme_light() : m.public_share_theme_dark()}
			onclick={() => setTheme(dark ? 'light' : 'dark')}
		>
			{#if dark}
				<Sun aria-hidden="true" class="size-5" />
			{:else}
				<Moon aria-hidden="true" class="size-5" />
			{/if}
		</IconButton>
	</header>

	<main class="flex-1">
		{#if recipe}
			<RecipeView {recipe} {imageSrc} onopenimage={openLightbox}>
				{#snippet belowSource()}
					<!-- Only while the owner lets the page name the sharer: the
					     profile head's band in miniature, a slim ribbon in their
					     color with their face on its left edge - their picture at
					     the share's own address when they have one, else their
					     initial in their strong tone, which the ribbon's own would
					     swallow. The page's ground cuts the face out of the ribbon,
					     as on the author band; a photo keeps PersonMark's ring and
					     lines that ring, not the face, up with the column. Under the source line,
					     the title block's last word: who handed the recipe on. -->
					{#if sharedBy}
						<p class="mt-4.5 flex pl-1">
							<span
								class="inline-flex min-h-8 min-w-0 items-center gap-2.5 rounded-pill pr-4 {userColorClasses(
									sharedBy.color
								)}"
							>
								<PersonMark
									person={{ id: '', ...sharedBy }}
									src={sharedBy.avatarUrl ?? null}
									size="ribbon"
									strong
									class="-my-1.5 ring-3 ring-background {sharedBy.avatarUrl ? '' : '-ml-1'}"
								/>
								<span class="min-w-0 py-1.5 text-body-sm wrap-break-word"
									>{#each sharedByParts as part, i (i)}{#if part === WHO}<b class="font-semibold"
												>{sharedBy.displayName}</b
											>{:else}{part}{/if}{/each}</span
								>
							</span>
						</p>
					{/if}
				{/snippet}
			</RecipeView>
		{:else}
			<p class="text-body text-text-muted">{m.public_share_unavailable()}</p>
		{/if}
	</main>

	<!-- An invitation for the stranger reading: where this cookbook comes from
	     and how to have one. Links off the SPA, so `resolve()` does not apply;
	     `noreferrer` on top of the page's own no-referrer policy, since the
	     token is in this page's address. -->
	{#if data.recipe?.attribution}
		<!-- eslint-disable svelte/no-navigation-without-resolve -->
		<!-- A hairline and room above set the bar apart from the recipe. On a
		     phone it is a centered stack - mark, title, sentence, a full-width
		     button and the source link under it - so no edge is left for the
		     links to line up with. From `sm` up it is a single row. -->
		<footer class="mt-14 border-t border-border pt-8">
			<div
				class="flex flex-col items-center gap-3 rounded-lg bg-surface px-5 py-5.5 text-center sm:flex-row sm:gap-3.5 sm:px-4.5 sm:py-3.5 sm:text-left"
			>
				<Lockup variant="mark" class="h-9 shrink-0 sm:h-7" />
				<p class="max-w-[36ch] text-body-sm sm:max-w-none sm:flex-1">
					<span class="mb-1 block font-display sm:mb-0 sm:inline"
						>{m.public_share_footer_title()}</span
					>
					<span class="text-text-muted">{m.public_share_footer()}</span>
				</p>
				<div
					class="mt-1 flex w-full flex-col items-center gap-3 sm:mt-0 sm:w-auto sm:flex-row sm:gap-4"
				>
					<a
						href={PROJECT_URL}
						rel="noreferrer noopener external"
						class="inline-flex h-10 w-full items-center justify-center gap-1.5 rounded-pill bg-primary hover-tint px-3 text-body-sm font-semibold text-primary-foreground transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] sm:h-8 sm:w-auto sm:text-caption"
					>
						{m.public_share_get()}
						<ArrowUpRight aria-hidden="true" class="size-3.5" />
					</a>
					<a
						href={SOURCE_URL}
						rel="noreferrer noopener external"
						class="inline-flex items-center gap-1 text-caption text-text-muted underline decoration-border underline-offset-4 transition hover:text-text hover:decoration-primary"
					>
						{m.public_share_source()}
						<ArrowUpRight aria-hidden="true" class="size-3.5" />
					</a>
				</div>
			</div>
		</footer>
		<!-- eslint-enable svelte/no-navigation-without-resolve -->
	{/if}
</div>

{#if recipe}
	<Lightbox
		bind:open={lightboxOpen}
		bind:index={lightboxIndex}
		recipeId={recipe.id}
		title={recipe.title}
		images={recipe.images}
		src={imageSrc}
	/>
{/if}
