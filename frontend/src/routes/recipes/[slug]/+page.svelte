<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { DropdownMenu } from 'bits-ui';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ChefHat from '@lucide/svelte/icons/chef-hat';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import Link from '@lucide/svelte/icons/link';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Share2 from '@lucide/svelte/icons/share-2';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import { toast } from 'svelte-sonner';
	import { deleteRecipe } from '$lib/api/recipes';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import IconButton from '$lib/components/ui/IconButton.svelte';
	import Lightbox from '$lib/components/ui/Lightbox.svelte';
	import TagChip from '$lib/components/ui/TagChip.svelte';
	import FavoriteStar from '$lib/components/recipe/FavoriteStar.svelte';
	import ImageGallery from '$lib/components/recipe/ImageGallery.svelte';
	import IngredientList from '$lib/components/recipe/IngredientList.svelte';
	import MetaPills from '$lib/components/recipe/MetaPills.svelte';
	import PlaceholderTile from '$lib/components/recipe/PlaceholderTile.svelte';
	import RecipeColophon from '$lib/components/recipe/RecipeColophon.svelte';
	import ServingsStepper from '$lib/components/recipe/ServingsStepper.svelte';
	import StepList from '$lib/components/recipe/StepList.svelte';
	import { clear as clearChecked } from '$lib/recipe/checked.svelte';
	import { clearServings, createServings } from '$lib/recipe/servings.svelte';
	import { m } from '$lib/paraglide/messages';
	import { shell } from '$lib/shell.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	// `recipe` (and everything derived from it below) can briefly read as
	// `undefined` while this page is torn down mid-navigation - some other
	// reactive update (the window scroll position resetting, `shell.actions`
	// re-rendering in the persistent top bar) can still land in that window.
	// Guarding each one here, once, keeps every reader below safe without
	// sprinkling `?.` through the whole template.
	const recipe = $derived(data.recipe);
	const editHref = $derived(recipe && resolve('/recipes/[slug]/edit', { slug: recipe.slug }));
	const cookHref = $derived(recipe && resolve('/recipes/[slug]/cook', { slug: recipe.slug }));
	// One store per recipe: `$derived` re-creates it when the page is reused
	// for another slug (command palette, back/forward), reading that recipe's
	// stored choice. Mutations go through `servings.set()`, not this binding.
	const servings = $derived(recipe && createServings(recipe.id, recipe.servings));

	let deleteOpen = $state(false);
	let lightboxOpen = $state(false);
	let lightboxIndex = $state(0);
	// Drives the mobile header's second state. The threshold is the cover's
	// bottom edge (24px page padding + 260px cover) minus the 64px header, so
	// the bar fills in exactly when it stops sitting on the image.
	let scrollY = $state(0);
	const scrolled = $derived(scrollY > 220);

	function openLightbox(index: number) {
		lightboxIndex = index;
		lightboxOpen = true;
	}

	// Both viewports render the same "..." menu; only the mobile one sits on
	// top of the cover image and needs a shadow.
	// The header's cook action: the same 40px circle as the back and menu
	// buttons, filled instead of outlined, so the title next to it keeps the
	// width a labeled pill would take. The labeled button stays in the flow.
	const cookTriggerClass =
		'inline-flex size-10 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98]';

	const menuTriggerClass =
		'inline-flex size-10 items-center justify-center rounded-full border border-border bg-surface text-text transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98]';

	async function copyLink() {
		await navigator.clipboard.writeText(location.href);
		toast.success(m.detail_link_copied());
	}

	async function handleDelete() {
		try {
			await deleteRecipe(recipe.id);
			clearChecked(recipe.id);
			clearServings(recipe.id);
			toast.success(m.detail_deleted());
			await goto(resolve('/'));
		} catch {
			toast.error(m.detail_delete_error());
		}
	}

	function goBack() {
		void goto(resolve('/'));
	}

	// Contributed to the desktop top bar via the `shell` store (see
	// `$lib/shell.svelte.ts`) instead of a prop, since `+layout.svelte` owns
	// `AppShell`/`TopBar` and doesn't know about this page's actions.
	//
	// `recipe` briefly reads as `undefined` while this page is being torn
	// down mid-navigation (a scroll event or another reactive update can
	// still land in that window), so this bails instead of throwing into
	// `data.recipe`.
	$effect(() => {
		if (!recipe) {
			return;
		}
		shell.breadcrumb = recipe.title;
		shell.actions = topBarActions;
		return () => {
			shell.breadcrumb = undefined;
			shell.actions = undefined;
		};
	});
</script>

<svelte:head>
	<title>{recipe ? `${recipe.title} · ${m.app_name()}` : m.app_name()}</title>
</svelte:head>

<!-- Edit and delete show only to those the server lets do them; see `Recipe.canEdit`. -->
{#snippet menuItems()}
	{#if recipe?.canEdit}
		<DropdownMenu.Item
			onSelect={() => goto(editHref)}
			class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-text transition hover:bg-background"
		>
			<Pencil class="size-4" aria-hidden="true" />
			{m.detail_edit()}
		</DropdownMenu.Item>
	{/if}
	<DropdownMenu.Item
		onSelect={copyLink}
		class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-text transition hover:bg-background"
	>
		<Link class="size-4" aria-hidden="true" />
		{m.detail_copy_link()}
	</DropdownMenu.Item>
	{#if recipe?.canDelete}
		<DropdownMenu.Item
			onSelect={() => (deleteOpen = true)}
			class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-destructive transition hover:bg-destructive-soft"
		>
			<Trash2 class="size-4" aria-hidden="true" />
			{m.common_delete()}
		</DropdownMenu.Item>
	{/if}
{/snippet}

{#snippet topBarActions()}
	{#if recipe?.canEdit}
		<Button variant="secondary" href={editHref}>
			{m.detail_edit()}
		</Button>
	{/if}
	<Button variant="primary" href={cookHref}>
		<ChefHat class="size-4" aria-hidden="true" />
		{m.detail_cook_mode()}
	</Button>
	<DropdownMenu.Root>
		<DropdownMenu.Trigger aria-label={m.detail_menu()} class={menuTriggerClass}>
			<Ellipsis class="size-5" aria-hidden="true" />
		</DropdownMenu.Trigger>
		<DropdownMenu.Portal>
			<DropdownMenu.Content
				preventScroll={false}
				sideOffset={8}
				align="end"
				class="w-52 rounded-2xl bg-surface p-2 shadow-dialog"
			>
				{@render menuItems()}
			</DropdownMenu.Content>
		</DropdownMenu.Portal>
	</DropdownMenu.Root>
{/snippet}

<svelte:window bind:scrollY />

{#if recipe}
	<!--
	Mobile header. It holds the controls that used to float on the cover: they
	stay put while the image scrolls away, and the bar then fades in its own
	background, the title and the cook-mode button. Over the cover the circles
	align with the image, not with the screen: the cover card's own 12px
	padding would otherwise leave them sitting on its frame on the left and
	right while clearing the image at the top. Filled in, the bar aligns with
	the page padding like every other row. Sharing steps aside in that
	state - "Link kopieren" is in the menu too - so the row never carries four
	controls at once. Over the image the bar itself must not swallow taps meant
	for the gallery, hence `pointer-events-none` on the empty middle.
-->
	<div
		class="fixed inset-x-0 top-0 z-40 flex gap-3 transition-all md:hidden {scrolled
			? 'h-16 items-center border-b border-border bg-background/90 px-5 backdrop-blur'
			: 'pointer-events-none h-28 items-start px-10 pt-11'}"
	>
		<IconButton
			label={m.detail_back()}
			onclick={goBack}
			class="pointer-events-auto {scrolled ? '' : 'shadow-card'}"
		>
			<ArrowLeft class="size-5" aria-hidden="true" />
		</IconButton>
		<p
			aria-hidden={!scrolled}
			class="flex-1 truncate font-display text-body font-medium transition-opacity {scrolled
				? 'opacity-100'
				: 'opacity-0'}"
		>
			{recipe?.title}
		</p>
		{#if scrolled}
			<a href={cookHref} aria-label={m.detail_cook_mode()} class={cookTriggerClass}>
				<ChefHat class="size-5" aria-hidden="true" />
			</a>
		{:else}
			<IconButton
				label={m.detail_copy_link()}
				onclick={copyLink}
				class="pointer-events-auto shadow-card"
			>
				<Share2 class="size-5" aria-hidden="true" />
			</IconButton>
		{/if}
		<DropdownMenu.Root>
			<DropdownMenu.Trigger
				aria-label={m.detail_menu()}
				class="{menuTriggerClass} pointer-events-auto {scrolled ? '' : 'shadow-card'}"
			>
				<Ellipsis class="size-5" aria-hidden="true" />
			</DropdownMenu.Trigger>
			<DropdownMenu.Portal>
				<!--
				`sideOffset` is measured from the 40px trigger, but once the bar
				has filled in, the menu has to clear the whole 64px bar: 20 puts
				it 8px below the bar's edge instead of 4px inside it. `z-50`
				(the same layer as dialogs and the lightbox) then keeps it above
				the `z-40` bar - Bits UI copies the content's computed z-index
				onto its floating wrapper, which is otherwise `auto` and loses,
				leaving the bar's backdrop blur to smear the menu's top edge.
			-->
				<DropdownMenu.Content
					preventScroll={false}
					sideOffset={scrolled ? 20 : 8}
					align="end"
					class="z-50 w-52 rounded-2xl bg-surface p-2 shadow-dialog"
				>
					{@render menuItems()}
				</DropdownMenu.Content>
			</DropdownMenu.Portal>
		</DropdownMenu.Root>
	</div>

	<article class="pt-6 md:pt-10">
		<div class="mb-6 h-[260px] overflow-hidden rounded-3xl bg-surface p-3 md:hidden">
			{#if recipe.images.length > 0}
				<ImageGallery
					recipeId={recipe.id}
					title={recipe.title}
					images={recipe.images}
					coverId={recipe.coverImageId}
					layout="mobile"
					onopen={openLightbox}
				/>
			{:else}
				<PlaceholderTile
					id={recipe.id}
					title={recipe.title}
					size="detail"
					class="size-full rounded-2xl"
				/>
			{/if}
		</div>

		<!--
			The cover column is `minmax(0,420px)`, not a flat `420px`: a `1fr`
			track cannot shrink past its own min-content, and here that floor
			is the longest word of the title at 52px - 335px for "Königsberger".
			With a rigid 420px beside it the row demanded 795px, which is more
			than this card has between 768px (where the two columns appear) and
			about 858px, and the cover hung over the edge of the page. Letting
			the cover give way keeps the title's floor intact; above 858px the
			cover still takes its full 420px and nothing about this row changes.

			The text column keeps its plain `1fr` on purpose. `minmax(0,1fr)`
			there would take the floor out from under the title instead, and
			the word would leave its column rather than the cover leaving the
			page - the same overflow, one step further in.
		-->
		<div class="md:grid md:grid-cols-[1fr_minmax(0,420px)] md:items-start md:gap-10">
			<div>
				{#if recipe.tags.length > 0}
					<div class="flex flex-wrap gap-2">
						{#each recipe.tags as tag (tag)}
							<TagChip label={tag} />
						{/each}
					</div>
				{/if}
				<div class="mt-3 flex items-start gap-3 md:mt-4">
					<h1 class="font-display text-display-md font-medium md:text-display-xl">
						{recipe.title}
					</h1>
					<div class="mt-1 shrink-0 md:mt-2">
						<FavoriteStar id={recipe.id} active={recipe.favorite} />
					</div>
				</div>
				{#if recipe.description}
					<p class="mt-3 text-body-lg text-text-muted">{recipe.description}</p>
				{/if}
				<div class="mt-5">
					<MetaPills
						prepMinutes={recipe.prepMinutes}
						cookMinutes={recipe.cookMinutes}
						sourceUrl={recipe.sourceUrl}
					>
						<ServingsStepper value={servings.value} onchange={(next) => servings.set(next)} />
						{#if servings.scaled}
							<button
								type="button"
								onclick={() => servings.reset()}
								class="text-body-sm font-medium text-primary underline-offset-4 transition hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
							>
								{m.servings_reset()}
							</button>
						{/if}
					</MetaPills>
				</div>
				<!-- On phones the primary action lives in the flow, right under the
			     servings it scales; the desktop top bar carries it there. -->
				<Button
					variant="primary"
					size="lg"
					href={cookHref}
					class="mt-6 w-full shadow-cta md:hidden"
				>
					<ChefHat class="size-5" aria-hidden="true" />
					{m.detail_cook_mode_start()}
				</Button>
			</div>
			<div class="hidden md:block">
				{#if recipe.images.length > 0}
					<ImageGallery
						recipeId={recipe.id}
						title={recipe.title}
						images={recipe.images}
						coverId={recipe.coverImageId}
						layout="desktop"
						onopen={openLightbox}
					/>
				{:else}
					<PlaceholderTile
						id={recipe.id}
						title={recipe.title}
						size="detail"
						class="aspect-[4/3] rounded-3xl shadow-cover"
					/>
				{/if}
			</div>
		</div>

		<div class="mt-8 grid gap-8 md:mt-10 md:grid-cols-[400px_1fr] md:gap-10">
			<section>
				<h2 class="mb-4 font-display text-heading font-medium">{m.recipe_ingredients()}</h2>
				<IngredientList
					recipeId={recipe.id}
					groups={recipe.ingredientGroups}
					servings={servings.value}
					baseServings={servings.base}
				/>
			</section>
			<section>
				<h2 class="mb-5 font-display text-heading font-medium">{m.recipe_steps()}</h2>
				<StepList
					steps={recipe.steps}
					groups={recipe.ingredientGroups}
					servings={servings.value}
					baseServings={servings.base}
				/>
			</section>
		</div>

		<RecipeColophon {recipe} />
	</article>

	<ConfirmDialog
		bind:open={deleteOpen}
		title={m.detail_delete_confirm_title()}
		text={m.detail_delete_confirm_text({ title: recipe.title })}
		confirmLabel={m.common_delete()}
		destructive
		onconfirm={handleDelete}
	/>

	<Lightbox
		bind:open={lightboxOpen}
		bind:index={lightboxIndex}
		recipeId={recipe.id}
		title={recipe.title}
		images={recipe.images}
	/>
{/if}
