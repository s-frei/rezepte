<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import ArrowDownWideNarrow from '@lucide/svelte/icons/arrow-down-wide-narrow';
	import Plus from '@lucide/svelte/icons/plus';
	import { toast } from 'svelte-sonner';
	import { afterNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { navigating, page } from '$app/state';
	import {
		listAuthors,
		listRecipes,
		listTags,
		type Author,
		type RecipeCard as RecipeCardData,
		type Tag
	} from '$lib/api/recipes';
	import Button from '$lib/components/ui/Button.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import InstallBanner from '$lib/components/install/InstallBanner.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import ActiveFilters from '$lib/components/recipe/ActiveFilters.svelte';
	import FilterPanel from '$lib/components/recipe/FilterPanel.svelte';
	import RecipeCard from '$lib/components/recipe/RecipeCard.svelte';
	import RecipeGrid from '$lib/components/recipe/RecipeGrid.svelte';
	import ResultCount from '$lib/components/recipe/ResultCount.svelte';
	import SearchBar from '$lib/components/recipe/SearchBar.svelte';
	import TagFilter from '$lib/components/recipe/TagFilter.svelte';
	import { formatDay } from '$lib/recipe/format';
	import { overviewViewState } from '$lib/recipe/overview-state';
	import { buildListQuery, isSort, parseListQuery, type Sort } from '$lib/recipe/query';
	import { applyRememberedSort, readSort, writeSort } from '$lib/recipe/sort-preference';
	import { importDialog } from '$lib/import.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// Seeded once from the initial load - after that, this component owns
	// them (searching/toggling tags updates them directly rather than
	// re-deriving from `data`, which would fight with those updates), and
	// `followUrl` below reseeds them when the URL changes for another reason.
	// `untrack` marks that one-time read as intentional.
	let q = $state(untrack(() => data.q));
	let tags = $state<string[]>(untrack(() => data.tags));
	let maxMinutes = $state(untrack(() => data.maxMinutes));
	let favoritesOnly = $state(untrack(() => data.favorites));
	let author = $state(untrack(() => data.author));
	let sort = $state<Sort>(untrack(() => data.sort));
	// The furthest page fetched for the current filters - the initial value
	// from the URL lets a shared/reloaded link resume where "Load more" left
	// off; searching or changing tags resets it back to 1.
	let pageNum = $state(untrack(() => data.page));

	// Read once: a page left open past midnight keeps yesterday's date,
	// which is fine for a running head.
	const today = new Date();
	// The local calendar day; toISOString() would give the UTC one, a day
	// off in the hours around midnight.
	const todayIso = [
		today.getFullYear(),
		String(today.getMonth() + 1).padStart(2, '0'),
		String(today.getDate()).padStart(2, '0')
	].join('-');

	let items = $state<RecipeCardData[]>([]);
	let total = $state(0);
	let allTags = $state<Tag[]>([]);
	/**
	 * The desktop row's tags while a search text is on, counted under it too:
	 * a pick from the row keeps the text, where a pick from the suggestions
	 * clears it, so the row's counts have to include it to stay true. `null`
	 * without a text, and the row then shares `allTags`.
	 */
	let textTags = $state<Tag[] | null>(null);
	// The chosen tags lead, in the order they were picked, like every other
	// active filter does - on the desktop row and at the top of the
	// suggestions - so picking one never sends it off somewhere along the
	// row. A chosen tag stays on offer even when the server no longer counts
	// it - a filter that empties the grid, a link to a tag since removed - or
	// nothing on the desktop row would say what is narrowing the grid, and
	// nothing in the suggestions would let it be dropped.
	const chosenFirst = (list: Tag[]) => [
		...tags.map((name) => list.find((tag) => tag.name === name) ?? { name, count: 0 }),
		...list.filter((tag) => !tags.includes(tag.name))
	];
	const tagOptions = $derived(chosenFirst(allTags));
	const rowOptions = $derived(chosenFirst(textTags ?? allTags));
	let authors = $state<Author[]>([]);

	let filterOpen = $state(false);

	let loading = $state(false);
	let loadingMore = $state(false);
	let showSkeleton = $state(false);

	let abortController: AbortController | null = null;
	let skeletonTimer: ReturnType<typeof setTimeout> | undefined;

	// The query string `syncUrl` last wrote. Not `$state`: it only tells the
	// page's own URL updates from someone else's, and nothing renders from it.
	// Seeded from the initial URL, so the `afterNavigate` that fires on mount
	// sees no change - `onMount` below does the first fetch.
	let lastQuery = untrack(() => buildListQuery(data));

	// True when a search term or any filter section narrows the list - the
	// one condition overviewViewState and ResultCount both need to tell "no
	// recipes at all" apart from "no recipes match". A later filter section
	// adds its own clause here rather than teaching either of those about it.
	const filtered = $derived(
		q !== '' || tags.length > 0 || maxMinutes > 0 || favoritesOnly || author !== ''
	);
	const viewState = $derived(
		overviewViewState({
			loading,
			filtered,
			itemCount: items.length,
			total
		})
	);
	const isEmpty = $derived(viewState.isEmpty);
	const isNoResults = $derived(viewState.isNoResults);
	const canLoadMore = $derived(viewState.canLoadMore);
	// Sum of everything narrowing the list: selected tags, an active time
	// filter, the favorites switch and an author, written as a sum so a
	// later filter adds its own term here. The tags are counted even though
	// they are set outside the panel - the count answers "how far is the
	// list narrowed", which is what "Reset all filters" inside the
	// panel clears. Sort is deliberately NOT a term, even though it has a
	// section in that panel: sorting returns the same recipes in a
	// different order - see the matching note on FilterPanel's `sort` prop.
	const filterCount = $derived(
		tags.length + (maxMinutes > 0 ? 1 : 0) + (favoritesOnly ? 1 : 0) + (author !== '' ? 1 : 0)
	);

	function focusFilters() {
		document.getElementById('overview-filters')?.focus();
	}

	// What both rows of active-filter chips get, the phone's and desktop's.
	const filterProps = $derived({
		maxMinutes,
		favoritesOnly,
		author,
		authors,
		onopen: () => (filterOpen = true),
		onmaxminutes: setMaxMinutes,
		onfavorites: setFavoritesOnly,
		onauthor: setAuthor
	});

	function syncUrl() {
		// A list that answers after the visitor has moved on (an import that
		// finished first) must not pull them back with its replaceState.
		if (page.route.id !== '/' || (navigating.to && navigating.to.route.id !== '/')) return;
		const qs = buildListQuery({
			q,
			tags,
			maxMinutes,
			favorites: favoritesOnly,
			author,
			sort,
			page: pageNum
		});
		lastQuery = qs;
		void goto(resolve(qs ? `/?${qs}` : '/'), {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}

	/**
	 * The tags are counted under the same filters as the grid, so every tag on
	 * offer narrows it and none empties it. The suggestions' list leaves the
	 * search text out - it is typed to find a tag, and a pick clears it - so
	 * only a change of the other filters refetches it; the desktop row keeps
	 * the text on a pick and gets a second list counted under it, only while
	 * there is one. The order is never a filter. A failure leaves the last
	 * tags standing - they only help to filter.
	 */
	function refreshTags() {
		const filters = { tags, maxMinutes, favorites: favoritesOnly, author };
		suggestedTags.load(filters);
		// Fetched on a phone too, where the row is hidden: it shows again,
		// with these counts, when the window widens.
		if (q) {
			rowTags.load({ ...filters, q });
		} else {
			rowTags.abort();
			textTags = null;
		}
	}

	/**
	 * One list of tags kept in step with its filters: a repeat of the filters
	 * it last fetched for is skipped, a newer fetch aborts the older one, and
	 * a failure forgets the filters so the next change retries. Each list has
	 * its own controller - "Load more" aborts the grid's, and must not take
	 * a tag fetch for the same filters down with it.
	 */
	function tagList(onload: (result: Tag[]) => void) {
		let controller: AbortController | null = null;
		let fetchedFor: string | null = null;
		return {
			load(params: Parameters<typeof listTags>[0] = {}) {
				const key = JSON.stringify(params);
				if (key === fetchedFor) {
					return;
				}
				fetchedFor = key;
				controller?.abort();
				const current = new AbortController();
				controller = current;
				void listTags(params, { signal: current.signal })
					.then((result) => {
						// An abort that lands after the response has resolved
						// does not stop it, so a superseded fetch checks itself.
						if (current === controller) {
							onload(result);
						}
					})
					.catch(() => {
						if (current === controller) {
							fetchedFor = null;
						}
					});
			},
			abort() {
				controller?.abort();
				fetchedFor = null;
			}
		};
	}
	const suggestedTags = tagList((result) => (allTags = result));
	const rowTags = tagList((result) => (textTags = result));

	async function fetchPage(pageToFetch: number, append: boolean) {
		abortController?.abort();
		const controller = new AbortController();
		abortController = controller;

		if (append) {
			loadingMore = true;
		} else {
			refreshTags();
			loading = true;
			clearTimeout(skeletonTimer);
			skeletonTimer = setTimeout(() => {
				if (loading) {
					showSkeleton = true;
				}
			}, 300);
		}

		try {
			const result = await listRecipes(
				{
					q: q || undefined,
					tags,
					maxMinutes,
					favorites: favoritesOnly,
					author: author || undefined,
					sort,
					page: pageToFetch
				},
				{ signal: controller.signal }
			);
			items = append ? [...items, ...result.items] : result.items;
			total = result.total;
			pageNum = result.page;
			syncUrl();
		} catch (err) {
			if (err instanceof DOMException && err.name === 'AbortError') {
				return;
			}
			toast.error(m.overview_load_error());
		} finally {
			// Guard against a superseded (aborted) request's `finally` clearing
			// the loading state of the request that replaced it.
			if (controller === abortController) {
				loading = false;
				loadingMore = false;
				clearTimeout(skeletonTimer);
				showSkeleton = false;
			}
		}
	}

	function handleSearch(value: string) {
		q = value;
		pageNum = 1;
		void fetchPage(1, false);
	}

	function toggleTag(name: string) {
		tags = tags.includes(name) ? tags.filter((t) => t !== name) : [...tags, name];
		pageNum = 1;
		void fetchPage(1, false);
	}

	function setMaxMinutes(value: number) {
		maxMinutes = value;
		pageNum = 1;
		void fetchPage(1, false);
	}

	function setFavoritesOnly(value: boolean) {
		favoritesOnly = value;
		pageNum = 1;
		void fetchPage(1, false);
	}

	function setAuthor(value: string) {
		author = value;
		pageNum = 1;
		void fetchPage(1, false);
	}

	// Only the sort control and the phone's sort chip come through here, so
	// this is where a choice of order is remembered; a link that names one
	// is applied by `load` and `followUrl` without being stored.
	function setSort(value: Sort, remember = true) {
		if (remember) {
			writeSort(value);
		}
		sort = value;
		pageNum = 1;
		void fetchPage(1, false);
	}

	// Sort is deliberately left alone: it isn't a filter (see the note on
	// filterCount), so "reset filters" doesn't touch it - someone who wants
	// the default order back uses the sort control itself, which is where
	// they set it.
	function resetFilters() {
		q = '';
		clearFilters();
	}

	// The phone's folded bar clears what it shows and keeps the search text,
	// which stands in the field above it.
	function clearFilters() {
		tags = [];
		maxMinutes = 0;
		favoritesOnly = false;
		author = '';
		pageNum = 1;
		void fetchPage(1, false);
	}

	function loadMore() {
		void fetchPage(pageNum + 1, true);
	}

	const sortOptions: { value: Sort; label: string }[] = [
		{ value: 'updated', label: m.overview_sort_updated() },
		{ value: 'created', label: m.overview_sort_created() },
		{ value: 'title', label: m.overview_sort_title() },
		{ value: 'tasty', label: m.overview_sort_tasty() }
	];

	function handleSort(value: string) {
		// Select's `value` is a plain string; narrowed back to Sort since
		// sortOptions only ever offers the known values as options.
		if (isSort(value)) {
			setSort(value);
		}
	}

	/**
	 * Follows a URL this page did not write itself: the shell's "Rezepte"
	 * links point back at `/`, the command palette's "all results" adds `?q=`,
	 * and the back button or a shared link restores an earlier filter. `load`
	 * re-runs for all of those, but the filter state lives in this component,
	 * so without reseeding it the grid would keep showing the old filter and
	 * the next `syncUrl()` would put it straight back into the URL.
	 *
	 * `buildListQuery` normalizes what it is handed (see `$lib/recipe/query`),
	 * so comparing it against the string `syncUrl` wrote recognizes the page's
	 * own updates and leaves them alone.
	 */
	function followUrl() {
		const parsed = parseListQuery(page.url);
		// The page's own write first: `syncUrl` leaves the default order out,
		// so a view in it, even one a `?sort=updated` link asked for, writes a
		// bare `/` that would otherwise take the remembered order below.
		if (buildListQuery(parsed) === lastQuery) {
			return;
		}
		// The same remembered order `load` applies, or arriving at a bare `/`
		// - the shell's "Recipes" link - would read as a change of order.
		const next = applyRememberedSort(parsed, page.url, readSort());
		if (buildListQuery(next) !== lastQuery) {
			q = next.q;
			tags = next.tags;
			maxMinutes = next.maxMinutes;
			favoritesOnly = next.favorites;
			author = next.author;
			sort = next.sort;
			pageNum = next.page;
			// The same page the URL names, so arriving here matches arriving
			// by reload.
			void fetchPage(next.page, false);
		}
	}

	afterNavigate(followUrl);

	onMount(() => {
		void fetchPage(pageNum, false);
		void listAuthors()
			.then((result) => (authors = result))
			.catch(() => {
				// Without it the panel simply shows no author section.
			});
		return () => {
			abortController?.abort();
			suggestedTags.abort();
			rowTags.abort();
		};
	});
</script>

<svelte:head><title>{m.app_name()}</title></svelte:head>

<section class="pt-5 md:pt-8">
	<InstallBanner />
	<!-- A running head, as at the top of a cookbook page: the section on the
	     left, a dotted leader like the ones in a table of contents, and on
	     the right how many recipes the list holds and today's date. The
	     count is ResultCount, so it turns into "3 found" under a filter;
	     there is no separate count line under the filters. The section name
	     is hidden from screen readers - it repeats the navigation, and the
	     headline right below says what the page is. -->
	<div
		class="mb-5 flex items-baseline gap-2.5 text-caption whitespace-nowrap text-text-muted md:mb-7"
	>
		<span aria-hidden="true" class="font-display text-body font-medium text-primary italic">
			{m.nav_recipes()}
		</span>
		<span
			aria-hidden="true"
			class="min-w-4 flex-1 -translate-y-1 border-b-2 border-dotted border-handle"
		></span>
		<ResultCount count={total} {filtered} />
		<span aria-hidden="true">·</span>
		<!-- A long weekday ("Donnerstag") does not fit a phone's line next to
		     the count, so below sm the date goes without it. -->
		<time datetime={todayIso}>
			<span class="sm:hidden">{formatDay(today, { weekday: false })}</span>
			<span class="hidden sm:inline">{formatDay(today)}</span>
		</time>
	</div>

	<h1 class="font-display text-display-sm font-medium md:text-display-lg">
		{m.overview_headline()}
	</h1>

	<!-- The same dotted leader closes the filters off from the grid. -->
	<!-- `gap`, not `space-y`: the phone's chip row stays in the DOM on desktop,
	     hidden, and as the last child it gave the tag row above a margin. -->
	<div class="mt-4 flex flex-col gap-3 border-b-2 border-dotted border-handle pb-4 md:mt-5">
		<!-- `relative` gives the search suggestions their width on phones: the
		     whole row, as wide as the recipe cards (see SearchBar). -->
		<div class="relative flex items-center gap-3">
			<SearchBar
				bind:value={q}
				onsearch={handleSearch}
				tags={tagOptions}
				active={tags}
				ontoggle={toggleTag}
				class="md:max-w-md"
			/>
			<FilterPanel
				bind:open={filterOpen}
				{maxMinutes}
				{favoritesOnly}
				{author}
				{authors}
				{sort}
				{filterCount}
				onmaxminutes={setMaxMinutes}
				onfavorites={setFavoritesOnly}
				onauthor={setAuthor}
				onsort={setSort}
				onreset={resetFilters}
			/>
			<!-- Desktop only: row 1 has room for search, the filter button and
			     this control side by side. On phones the same control moves
			     into the filter panel as its own section instead (see
			     FilterPanel), since row 1 there fits only the search field and
			     the filter button. Visibility is toggled on this wrapper, not
			     via a "hidden ... md:inline-flex" class handed to Select itself:
			     Select's trigger already hardcodes "inline-flex" ahead of the
			     class prop it is given, and that plain (unprefixed) utility won,
			     leaving the trigger visible - and row 1 overflowing - on phones
			     too. A wrapper's display:none hides every descendant regardless
			     of the descendant's own display value, which sidesteps the
			     utility-order question entirely. -->
			<div class="hidden shrink-0 md:block">
				<Select
					value={sort}
					options={sortOptions}
					label={m.overview_sort_label()}
					onchange={handleSort}
					class="bg-surface text-text"
				>
					{#snippet icon()}
						<ArrowDownWideNarrow class="size-4 shrink-0" aria-hidden="true" />
					{/snippet}
				</Select>
			</div>
		</div>
		<!-- What narrows the grid stays in view: the filter panel's choices as
		     light chips, which go back to the panel, then the tags. Desktop
		     has the room to spell everything out ahead of its row of the tags
		     still on offer. On a phone that row cost a line folded and a
		     screen unfolded, so tags are found through the search's
		     suggestions there, only the chosen ones show, and chips and tags
		     fold into one bar once the row runs out of room. -->
		<!-- The panel's filters lie in a faint tray of their own, which sets
		     them apart from the tags on offer without a rule floating between
		     the two; as the row's first item, an unfolded row wraps under it. -->
		{#snippet panelFilters()}
			<ActiveFilters
				{...filterProps}
				class="flex gap-1 rounded-pill bg-text/7 p-0.5"
				focusfallback={focusFilters}
			/>
		{/snippet}
		<div class="hidden md:flex">
			<TagFilter
				tags={rowOptions}
				active={tags}
				ontoggle={toggleTag}
				leading={filterCount > tags.length ? panelFilters : undefined}
			/>
		</div>
		{#if filterCount > 0 || sort !== 'updated'}
			<ActiveFilters
				{...filterProps}
				sortLabel={sort === 'updated'
					? ''
					: (sortOptions.find((option) => option.value === sort)?.label ?? '')}
				onsortreset={() => {
					// An order a link brought along is dropped for this view only;
					// the reader's own remembered one is let go of.
					setSort('updated', sort === readSort());
				}}
				{tags}
				ontagremove={toggleTag}
				onreset={clearFilters}
				collapsible
				class="flex flex-wrap gap-2 md:hidden"
				focusfallback={focusFilters}
			/>
		{/if}
	</div>

	<div class="mt-7 md:mt-9">
		{#if showSkeleton}
			<RecipeGrid>
				{#each Array.from({ length: 8 }, (_, i) => i) as i (i)}
					<div class="rounded-xl bg-surface p-1.5 shadow-card md:rounded-2xl md:p-2">
						<Skeleton class="aspect-square rounded-lg" />
						<div class="space-y-2 px-2.5 pt-2.5 pb-4">
							<Skeleton class="h-3.5 w-3/4 rounded-sm" />
							<Skeleton class="h-2.5 w-1/2 rounded-sm" />
						</div>
					</div>
				{/each}
			</RecipeGrid>
		{:else if isEmpty}
			<EmptyState title={m.overview_empty_title()} text={m.overview_empty_text()}>
				<Button variant="primary" href={resolve('/recipes/new')}>
					<Plus class="size-4" aria-hidden="true" />
					{m.overview_new_recipe()}
				</Button>
				<Button variant="secondary" onclick={() => (importDialog.open = true)}>
					{m.overview_import()}
				</Button>
			</EmptyState>
		{:else if isNoResults}
			<!-- Only a typed term gets quoted back. Tags, time and favorites
			     narrow the list just as well, but naming them in the same
			     sentence would quote something nobody typed - and with none of
			     them typed at all, the sentence used to close around an empty
			     pair of quotation marks. Which tags are on is legible anyway:
			     their chips sit above the grid with the active ones marked. -->
			<EmptyState
				title={m.overview_no_results_title()}
				text={q ? m.overview_no_results_text({ query: q }) : m.overview_no_results_filtered()}
			>
				<Button variant="secondary" onclick={resetFilters}>{m.overview_reset_filters()}</Button>
				<!-- Carrying the term over is a route param the editor reads, not
				     an API call, so this opens the editor with the search term
				     already in the title. Without a term there is nothing to
				     carry and the same button is simply the plain way on to a
				     new recipe - an empty result is still a reason to write one,
				     which is why it stays rather than disappearing. -->
				<Button
					variant="accent"
					href={q
						? resolve(`/recipes/new?title=${encodeURIComponent(q)}`)
						: resolve('/recipes/new')}
				>
					{q ? m.overview_create_from_search() : m.overview_new_recipe()}
				</Button>
			</EmptyState>
		{:else}
			<RecipeGrid>
				{#each items as recipe (recipe.id)}
					<RecipeCard {recipe} />
				{/each}
			</RecipeGrid>
			{#if canLoadMore}
				<div class="mt-8 flex justify-center">
					<!-- Also disabled while a filter fetch is in flight: "Load more"
					     would abort it and append the new filter's page N+1 to the
					     items the old filter left behind. -->
					<Button variant="secondary" onclick={loadMore} disabled={loading || loadingMore}>
						{m.overview_load_more()}
					</Button>
				</div>
			{/if}
		{/if}
	</div>
</section>
