<script lang="ts">
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import { imageUrl } from '#lib/api/recipes.js';
	import type { PlanEntry } from '#lib/api/mealplan.js';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { recipeHref } from '#lib/mealplan/entries.js';
	import { m } from '#lib/paraglide/messages.js';
	import { initialFor, tintFor } from '#lib/recipe/placeholder.js';
	import EntryMenu from './EntryMenu.svelte';

	/**
	 * One planned meal. `column` is the desktop's small recipe card, photo on
	 * top; `row` the compact line of a day row or a phone's day card, the
	 * thumbnail beside the title; `chip` the tray strip's pill. A free text
	 * is a dashed card in every layout, its words in italic Literata.
	 */
	let {
		entry,
		layout,
		onmove,
		onservings,
		onremove
	}: {
		entry: PlanEntry;
		layout: 'column' | 'row' | 'chip';
		onmove: () => void;
		onservings: (servings: number) => void;
		onremove: () => void;
	} = $props();

	const href = $derived(recipeHref(entry));
	const list = $derived(entry.lists[0]);
	const servings = $derived(
		entry.servings === null
			? ''
			: entry.servings === 1
				? m.plan_servings_one()
				: m.plan_servings({ count: entry.servings })
	);
	const by = $derived(entry.createdBy ? m.plan_by({ name: entry.createdBy.displayName }) : '');

	// A link's own long-press menu and a word's selection would take over
	// from the drag a held finger starts; the card has its ⋯ instead.
	const still =
		'select-none [-webkit-touch-callout:none] [-webkit-tap-highlight-color:transparent]';
	// The whole card opens the recipe: the title's link stretched over it.
	const stretched =
		'outline-none after:absolute after:inset-0 after:rounded-[inherit] focus-visible:after:outline-2 focus-visible:after:outline-offset-2 focus-visible:after:outline-primary';
	// The tap the zone repeats after a touch that never became a drag
	// (svelte-dnd-action's own click) would open the page a second time.
	const once = (event: MouseEvent) => {
		if (!event.isTrusted) event.preventDefault();
	};
</script>

{#snippet title(cls: string)}
	{#if href}
		<a {href} draggable="false" onclick={once} class="{cls} {stretched}">{entry.title}</a>
	{:else}
		<span class={cls}>{entry.title}</span>
	{/if}
{/snippet}

{#snippet cover(cls: string, letter: string)}
	{#if entry.recipeId && entry.photo}
		<img
			src={imageUrl(entry.recipeId, entry.photo, 'thumb')}
			alt=""
			loading="lazy"
			draggable="false"
			class="block object-cover [-webkit-user-drag:none] {cls}"
		/>
	{:else}
		<span
			aria-hidden="true"
			class="flex items-center justify-center font-display font-medium text-primary italic {tintFor(
				entry.recipeId ?? entry.id
			)} {letter} {cls}">{initialFor(entry.title)}</span
		>
	{/if}
{/snippet}

{#snippet mark(cls: string)}
	{#if entry.createdBy}
		<span title={by} class="flex shrink-0 {cls}">
			<PersonMark person={entry.createdBy} size="xs" />
			<span class="sr-only">{by}</span>
		</span>
	{/if}
{/snippet}

{#snippet onList()}
	{#if list}
		<span
			title={entry.lists.map((l) => l.name).join(', ')}
			class="inline-flex min-w-0 items-center gap-1 text-caption font-semibold text-success-foreground"
		>
			<ShoppingBasket class="size-3.5 shrink-0" aria-hidden="true" />
			<span class="sr-only">{m.plan_on_list({ name: list.name })}</span>
			<span aria-hidden="true" class="truncate">{list.name}</span>
		</span>
	{/if}
{/snippet}

{#snippet menu(trigger: string)}
	<EntryMenu {entry} {trigger} {onmove} {onservings} {onremove} />
{/snippet}

{#if entry.recipeId === null && entry.servings === null}
	<!-- A note: dashed, italic, the adder's mark beside it. -->
	<div
		class="group relative flex min-h-11 gap-2 rounded-lg border-[1.5px] border-dashed border-handle {still} {layout ===
		'column'
			? 'items-start py-2.5 pr-1.5 pl-3'
			: 'items-center pl-3.5'}"
	>
		<span
			class="min-w-0 font-display break-words [hyphens:none] text-text italic {layout === 'column'
				? 'flex-1 text-body-sm leading-snug'
				: 'text-body leading-snug'}">{entry.title}</span
		>
		{#if layout === 'column'}
			<!-- The mark over the ⋯, so a column's narrow card leaves the words their
			     width. On a pointer that hovers, the ⋯ waits for the card; focus shows it too. -->
			<span class="-my-1 flex shrink-0 flex-col items-center gap-1">
				{@render mark('mt-1')}
				{@render menu(
					'size-7 text-text-muted hover:bg-background [@media(hover:hover)]:opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 focus-visible:opacity-100 aria-expanded:opacity-100'
				)}
			</span>
		{:else}
			{@render mark('mr-auto')}
			{@render menu('size-11 text-text-muted md:size-9 hover:bg-background')}
		{/if}
	</div>
{:else if layout === 'column'}
	<article class="relative overflow-hidden rounded-xl bg-surface shadow-card {still}">
		<div class="relative">
			{@render cover('aspect-4/3 w-full', 'text-display-sm')}
			{@render mark('absolute -bottom-2 left-2 rounded-full ring-2 ring-surface')}
		</div>
		<div class="flex flex-col gap-0.5 px-2.5 pt-3 pb-2.5">
			{@render title('text-body-sm leading-snug font-semibold text-text [hyphens:none]')}
			<span class="text-caption whitespace-nowrap text-text-muted">{servings}</span>
			<span class="mt-0.5 flex min-w-0">{@render onList()}</span>
		</div>
		<div class="absolute top-1.5 right-1.5 z-10">
			{@render menu('size-7 bg-surface text-text shadow-card hover:bg-background')}
		</div>
	</article>
{:else}
	<article
		class="relative flex min-h-13 items-center gap-3 rounded-lg {still} {layout === 'chip'
			? 'md:min-h-0 md:bg-background md:py-1 md:pl-1'
			: ''}"
	>
		<span class="relative shrink-0">
			{@render cover(`rounded-md ${layout === 'chip' ? 'size-11' : 'size-12'}`, 'text-heading')}
			{@render mark('absolute -right-1 -bottom-1 rounded-full ring-2 ring-surface')}
		</span>
		<span class="flex min-w-0 flex-1 {layout === 'chip' ? 'flex-col items-start' : 'flex-col'}">
			{@render title(
				`text-body-sm leading-snug font-semibold text-text [hyphens:none] ${layout === 'chip' ? 'md:whitespace-nowrap' : ''}`
			)}
			<span class="flex flex-wrap items-center gap-x-2.5">
				<span class="text-caption whitespace-nowrap text-text-muted">{servings}</span>
				{@render onList()}
			</span>
		</span>
		<span class="relative z-10 flex">
			{@render menu(
				`text-text-muted hover:bg-surface ${layout === 'chip' ? 'size-11 md:size-8' : 'size-11 md:size-9'}`
			)}
		</span>
	</article>
{/if}
