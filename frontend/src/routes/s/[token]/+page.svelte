<script lang="ts">
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import Moon from '@lucide/svelte/icons/moon';
	import Sun from '@lucide/svelte/icons/sun';
	import { MediaQuery } from 'svelte/reactivity';
	import { publicImageUrl } from '$lib/api/public';
	import { PROJECT_URL, SOURCE_URL } from '$lib/docs';
	import type { ImageVariant, RecipeContent } from '$lib/api/recipes';
	import Lockup from '$lib/components/brand/Lockup.svelte';
	import RecipeView from '$lib/components/recipe/RecipeView.svelte';
	import IconButton from '$lib/components/ui/IconButton.svelte';
	import Lightbox from '$lib/components/ui/Lightbox.svelte';
	import { m } from '$lib/paraglide/messages';
	import { setTheme, theme } from '$lib/theme.svelte';
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

<div class="mx-auto flex min-h-dvh max-w-[1280px] flex-col px-5 py-8 md:px-8 md:py-10">
	<header class="mb-8 flex items-center justify-between gap-4">
		<Lockup variant="compact" label={m.app_name()} class="h-7" />
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
			<RecipeView {recipe} {imageSrc} onopenimage={openLightbox} />
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
				<Lockup variant="mark" class="size-9 shrink-0 sm:size-7" />
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
						class="inline-flex h-10 w-full items-center justify-center gap-1.5 rounded-pill bg-primary px-3 text-body-sm font-semibold text-primary-foreground transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] sm:h-8 sm:w-auto sm:text-caption"
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
