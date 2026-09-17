<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { DropdownMenu } from 'bits-ui';
	import ArrowLeft from 'lucide-svelte/icons/arrow-left';
	import ChefHat from 'lucide-svelte/icons/chef-hat';
	import Ellipsis from 'lucide-svelte/icons/ellipsis';
	import Link from 'lucide-svelte/icons/link';
	import Pencil from 'lucide-svelte/icons/pencil';
	import Share2 from 'lucide-svelte/icons/share-2';
	import Trash2 from 'lucide-svelte/icons/trash-2';
	import { toast } from 'svelte-sonner';
	import { deleteRecipe } from '$lib/api/recipes';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import IconButton from '$lib/components/ui/IconButton.svelte';
	import TagChip from '$lib/components/ui/TagChip.svelte';
	import IngredientList from '$lib/components/recipe/IngredientList.svelte';
	import MetaPills from '$lib/components/recipe/MetaPills.svelte';
	import PlaceholderTile from '$lib/components/recipe/PlaceholderTile.svelte';
	import StepList from '$lib/components/recipe/StepList.svelte';
	import { clear as clearChecked } from '$lib/recipe/checked.svelte';
	import { m } from '$lib/paraglide/messages';
	import { shell } from '$lib/shell.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const recipe = $derived(data.recipe);
	const editHref = $derived(resolve('/recipes/[slug]/edit', { slug: recipe.slug }));

	let deleteOpen = $state(false);

	// Both viewports render the same "..." menu; only the mobile one sits on
	// top of the cover image and needs a shadow.
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
	$effect(() => {
		shell.breadcrumb = recipe.title;
		shell.actions = topBarActions;
		return () => {
			shell.breadcrumb = undefined;
			shell.actions = undefined;
		};
	});
</script>

{#snippet menuItems()}
	<DropdownMenu.Item
		onSelect={() => goto(editHref)}
		class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-text transition hover:bg-background"
	>
		<Pencil class="size-4" aria-hidden="true" />
		{m.detail_edit()}
	</DropdownMenu.Item>
	<DropdownMenu.Item
		onSelect={copyLink}
		class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-text transition hover:bg-background"
	>
		<Link class="size-4" aria-hidden="true" />
		{m.detail_copy_link()}
	</DropdownMenu.Item>
	<DropdownMenu.Item
		onSelect={() => (deleteOpen = true)}
		class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-destructive transition hover:bg-destructive-soft"
	>
		<Trash2 class="size-4" aria-hidden="true" />
		{m.common_delete()}
	</DropdownMenu.Item>
{/snippet}

{#snippet topBarActions()}
	<Button variant="secondary" href={editHref}>
		{m.detail_edit()}
	</Button>
	<Button variant="primary" disabled title={m.common_coming_soon()}>
		<ChefHat class="size-4" aria-hidden="true" />
		{m.detail_cook_mode()}
	</Button>
	<DropdownMenu.Root>
		<DropdownMenu.Trigger aria-label={m.detail_menu()} class={menuTriggerClass}>
			<Ellipsis class="size-5" aria-hidden="true" />
		</DropdownMenu.Trigger>
		<DropdownMenu.Portal>
			<DropdownMenu.Content
				sideOffset={8}
				align="end"
				class="w-52 rounded-2xl bg-surface p-2 shadow-dialog"
			>
				{@render menuItems()}
			</DropdownMenu.Content>
		</DropdownMenu.Portal>
	</DropdownMenu.Root>
{/snippet}

<article class="pt-6 pb-20 md:pt-10 md:pb-0">
	<div class="relative mb-6 md:hidden">
		<div class="h-[260px] overflow-hidden rounded-3xl bg-surface p-3">
			<PlaceholderTile
				id={recipe.id}
				title={recipe.title}
				size="detail"
				class="size-full rounded-2xl"
			/>
		</div>
		<div class="absolute inset-x-5 top-5 flex items-center justify-between">
			<IconButton label={m.detail_back()} onclick={goBack} class="shadow-card">
				<ArrowLeft class="size-5" aria-hidden="true" />
			</IconButton>
			<div class="flex gap-2">
				<IconButton label={m.detail_copy_link()} onclick={copyLink} class="shadow-card">
					<Share2 class="size-5" aria-hidden="true" />
				</IconButton>
				<DropdownMenu.Root>
					<DropdownMenu.Trigger aria-label={m.detail_menu()} class="{menuTriggerClass} shadow-card">
						<Ellipsis class="size-5" aria-hidden="true" />
					</DropdownMenu.Trigger>
					<DropdownMenu.Portal>
						<DropdownMenu.Content
							sideOffset={8}
							align="end"
							class="w-52 rounded-2xl bg-surface p-2 shadow-dialog"
						>
							{@render menuItems()}
						</DropdownMenu.Content>
					</DropdownMenu.Portal>
				</DropdownMenu.Root>
			</div>
		</div>
	</div>

	<div class="md:grid md:grid-cols-[1fr_420px] md:items-start md:gap-10">
		<div>
			{#if recipe.tags.length > 0}
				<div class="flex flex-wrap gap-2">
					{#each recipe.tags as tag (tag)}
						<TagChip label={tag} />
					{/each}
				</div>
			{/if}
			<h1 class="mt-3 font-display text-display-md font-medium md:mt-4 md:text-display-xl">
				{recipe.title}
			</h1>
			{#if recipe.description}
				<p class="mt-3 text-body-lg text-text-muted">{recipe.description}</p>
			{/if}
			<div class="mt-5">
				<MetaPills
					servings={recipe.servings}
					prepMinutes={recipe.prepMinutes}
					cookMinutes={recipe.cookMinutes}
					sourceUrl={recipe.sourceUrl}
				/>
			</div>
		</div>
		<div class="hidden md:block">
			<PlaceholderTile
				id={recipe.id}
				title={recipe.title}
				size="detail"
				class="aspect-[4/3] rounded-3xl shadow-cover"
			/>
		</div>
	</div>

	<div class="mt-8 grid gap-8 md:mt-10 md:grid-cols-[400px_1fr] md:gap-10">
		<section>
			<h2 class="mb-4 font-display text-heading font-medium">{m.recipe_ingredients()}</h2>
			<IngredientList recipeId={recipe.id} groups={recipe.ingredientGroups} />
		</section>
		<section>
			<h2 class="mb-5 font-display text-heading font-medium">{m.recipe_steps()}</h2>
			<StepList steps={recipe.steps} />
		</section>
	</div>
</article>

<div
	class="fixed inset-x-0 bottom-24 z-20 flex justify-center bg-gradient-to-t from-background via-background to-transparent px-4 pt-6 pb-2 md:hidden"
>
	<Button
		variant="primary"
		size="lg"
		disabled
		title={m.common_coming_soon()}
		class="w-full shadow-cta"
	>
		{m.detail_cook_mode_start()}
	</Button>
</div>

<ConfirmDialog
	bind:open={deleteOpen}
	title={m.detail_delete_confirm_title()}
	text={m.detail_delete_confirm_text({ title: recipe.title })}
	confirmLabel={m.common_delete()}
	destructive
	onconfirm={handleDelete}
/>
