<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { DropdownMenu } from 'bits-ui';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ChefHat from '@lucide/svelte/icons/chef-hat';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import Globe from '@lucide/svelte/icons/globe';
	import Link from '@lucide/svelte/icons/link';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Share2 from '@lucide/svelte/icons/share-2';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import { toast } from 'svelte-sonner';
	import { markCommentsSeen } from '$lib/api/comments';
	import { deleteRecipe } from '$lib/api/recipes';
	import type { PublicShare } from '$lib/api/shares';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import IconButton from '$lib/components/ui/IconButton.svelte';
	import Lightbox from '$lib/components/ui/Lightbox.svelte';
	import KitchenDiary from '$lib/components/recipe/KitchenDiary.svelte';
	import FavoriteStar from '$lib/components/recipe/FavoriteStar.svelte';
	import PublicShareDialog from '$lib/components/recipe/PublicShareDialog.svelte';
	import RecipeColophon from '$lib/components/recipe/RecipeColophon.svelte';
	import RecipeView from '$lib/components/recipe/RecipeView.svelte';
	import ShareSheet from '$lib/components/recipe/ShareSheet.svelte';
	import TastyButton from '$lib/components/recipe/TastyButton.svelte';
	import TastyPeople from '$lib/components/recipe/TastyPeople.svelte';
	import { session } from '$lib/auth.svelte';
	import { clear as clearChecked } from '$lib/recipe/checked.svelte';
	import { clearServings } from '$lib/recipe/servings.svelte';
	import { copyLink, ShareLink } from '$lib/recipe/share.svelte';
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
	const settings = $derived(data.settings);

	// The people behind the tasty count. Derived from the loaded recipe, and
	// written over when the reader marks or unmarks it, so their own circle
	// follows the button without reloading the recipe.
	let tastyBy = $derived(recipe?.tastyBy ?? []);
	const ownRecipe = $derived(recipe && session.user?.id === recipe.createdBy.id);

	function onTasty(active: boolean) {
		const me = session.user;
		if (!me) {
			return;
		}
		tastyBy = active
			? [
					...tastyBy,
					{
						id: me.id,
						username: me.username,
						displayName: me.displayName,
						color: me.color,
						avatarId: me.avatarId
					}
				]
			: tastyBy.filter((person) => person.id !== me.id);
	}

	// The caller's own public link, if any. A *writable* $derived: reading it
	// tracks `data.share` (so navigating to another recipe resets it), but
	// `PublicShareDialog` also reassigns it in place on create/revoke, which
	// overrides that until `data.share` itself changes again - exactly what
	// lets the menu item, the tag-row marker and the colophon line below update
	// without a reload.
	let share: PublicShare | null = $derived(data.share);
	let shareDialogOpen = $state(false);
	let passOnOpen = $state(false);
	// Public sharing is on and this member may use it.
	// With an existing link the menu item and the marker show regardless of
	// either: they open the dialog, which shows the link as paused if either
	// has since gone off, with Revoke still available.
	const canCreateShare = $derived(
		Boolean(settings?.publicShares) && Boolean(session.user?.canSharePublicly)
	);
	const showShareMenuItem = $derived(canCreateShare || share !== null);

	// The recipe's comments, here rather than in `KitchenDiary` so the
	// tab can count them. Writable deriveds: a new load reseeds both.
	let diaryEntries = $derived(data.comments ?? []);
	// Opening the tab marks what was loaded as seen, once a visit: an entry
	// written since the load stays new. The entries keep their New flags for
	// this visit; only the tab's dot goes.
	let diarySeen = $derived(data.comments === null);

	function openDiary() {
		if (diarySeen) return;
		diarySeen = true;
		const newest = data.comments?.at(-1)?.id;
		if (newest) markCommentsSeen(recipe.id, newest).catch(() => {});
	}

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

	// The address "Copy link" and the share button hand out; see
	// `$lib/recipe/share.svelte.ts`. One per recipe: re-created when the page
	// is reused for another slug, released when it goes.
	let shareable: ShareLink | null = null;
	$effect(() => {
		const id = recipe?.id;
		if (!id) {
			return;
		}
		const link = new ShareLink(id);
		shareable = link;
		return () => link.destroy();
	});

	function copyRecipeLink() {
		return copyLink(shareable?.url ?? location.href);
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
		onSelect={copyRecipeLink}
		class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-text transition hover:bg-background"
	>
		<Link class="size-4" aria-hidden="true" />
		{m.detail_copy_link()}
	</DropdownMenu.Item>
	<DropdownMenu.Item
		onSelect={() => (passOnOpen = true)}
		class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-text transition hover:bg-background"
	>
		<Share2 class="size-4" aria-hidden="true" />
		{m.share_pass_on()}
	</DropdownMenu.Item>
	{#if showShareMenuItem}
		<DropdownMenu.Item
			onSelect={() => (shareDialogOpen = true)}
			class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-text transition hover:bg-background"
		>
			<Globe class="size-4" aria-hidden="true" />
			{share ? m.public_share_menu_manage() : m.public_share_menu_create()}
		</DropdownMenu.Item>
	{/if}
	{#if recipe?.canDelete}
		<!-- The one destructive item sits apart from the rest. -->
		<DropdownMenu.Separator class="mx-1 my-1.5 h-px bg-popover-border" />
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
	<Button variant="secondary" onclick={() => (passOnOpen = true)}>
		<Share2 class="size-4" aria-hidden="true" />
		{m.share_pass_on()}
	</Button>
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
				class="w-52 rounded-2xl border border-popover-border bg-popover p-2 shadow-dialog"
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
				label={m.detail_share()}
				onclick={() => (passOnOpen = true)}
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
					class="z-50 w-52 rounded-2xl border border-popover-border bg-popover p-2 shadow-dialog"
				>
					{@render menuItems()}
				</DropdownMenu.Content>
			</DropdownMenu.Portal>
		</DropdownMenu.Root>
	</div>

	<!-- Without the entries (their load failed) the steps keep a plain heading. -->
	{#snippet diary()}
		<!-- RecipeView keys the tabs, and so this, on the recipe. -->
		<KitchenDiary recipeId={recipe.id} bind:entries={diaryEntries} />
	{/snippet}
	<article class="pt-6 md:pt-10">
		<RecipeView
			{recipe}
			onopenimage={openLightbox}
			diaryCount={diaryEntries.length}
			diaryNew={!diarySeen && diaryEntries.some((e) => e.new)}
			diary={data.comments ? diary : undefined}
			ondiaryopen={openDiary}
		>
			{#snippet byline()}
				<!-- Star and heart sit under the title, outside any link, and this
				     is where a phone sets both: its cards leave the photo to the
				     photo. The heart's count and the people it counts read as one
				     line, like a dedication under a cookbook's title. -->
				<FavoriteStar id={recipe.id} active={recipe.favorite} />
				{#if !ownRecipe || recipe.tastyCount > 0}
					<TastyButton
						id={recipe.id}
						active={recipe.tasty}
						count={recipe.tastyCount}
						readonly={ownRecipe}
						onchange={onTasty}
					/>
				{/if}
				{#if tastyBy.length > 0}
					<div class="ml-1 flex items-center gap-2 text-caption text-text-muted">
						<span>{m.recipe_tasty_by()}</span>
						<TastyPeople people={tastyBy} />
					</div>
				{/if}
			{/snippet}
			{#snippet kicker()}
				<!-- The viewer's own public link, at a glance: in the tag row rather
				     than beside the title, so it wraps with the tags instead of
				     pushing the title into the photo. Outlined, so it never reads as
				     one more tag; the colophon below carries the details. -->
				{#if share}
					<button
						type="button"
						onclick={() => (shareDialogOpen = true)}
						class="inline-flex items-center gap-1.5 rounded-pill border px-3 py-0.5 text-caption font-semibold transition hover:bg-background focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {share.status ===
						'active'
							? 'border-primary text-primary'
							: 'border-border text-text-muted'}"
					>
						<Globe class="size-3.5" aria-hidden="true" />
						{m.public_share_marker()}
					</button>
				{/if}
			{/snippet}
			{#snippet belowMeta()}
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
			{/snippet}
			{#snippet footer()}
				<RecipeColophon {recipe} {share} onmanageshare={() => (shareDialogOpen = true)} />
			{/snippet}
		</RecipeView>
	</article>

	<ConfirmDialog
		bind:open={deleteOpen}
		title={m.detail_delete_confirm_title()}
		text={m.detail_delete_confirm_text({ title: recipe.title })}
		confirmLabel={m.common_delete()}
		destructive
		onconfirm={handleDelete}
	/>

	<PublicShareDialog
		bind:open={shareDialogOpen}
		recipeId={recipe.id}
		recipeTitle={recipe.title}
		defaultDays={settings?.publicShareDefaultDays ?? null}
		maxDays={settings?.publicShareMaxDays ?? null}
		bind:share
	/>

	<ShareSheet
		bind:open={passOnOpen}
		{recipe}
		attribution={settings?.publicShareAttribution ?? true}
		linkUrl={() => shareable?.url ?? location.href}
	/>

	<Lightbox
		bind:open={lightboxOpen}
		bind:index={lightboxIndex}
		recipeId={recipe.id}
		title={recipe.title}
		images={recipe.images}
	/>
{/if}
