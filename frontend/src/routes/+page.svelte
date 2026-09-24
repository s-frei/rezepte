<script lang="ts">
	import { onMount, tick, untrack } from 'svelte';
	import ArrowDownWideNarrow from '@lucide/svelte/icons/arrow-down-wide-narrow';
	import Plus from '@lucide/svelte/icons/plus';
	import { toast } from 'svelte-sonner';
	import { afterNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
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
	import Select from '$lib/components/ui/Select.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import FilterPanel from '$lib/components/recipe/FilterPanel.svelte';
	import RecipeCard from '$lib/components/recipe/RecipeCard.svelte';
	import RecipeGrid from '$lib/components/recipe/RecipeGrid.svelte';
	import ResultCount from '$lib/components/recipe/ResultCount.svelte';
	import SearchBar from '$lib/components/recipe/SearchBar.svelte';
	import TagFilter from '$lib/components/recipe/TagFilter.svelte';
	import { overviewViewState } from '$lib/recipe/overview-state';
	import { buildListQuery, isSort, parseListQuery, type Sort } from '$lib/recipe/query';
	import { m } from '$lib/paraglide/messages';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	/** Id of the search field, so `?focus=search` can hand it the caret. */
	const SEARCH_FIELD_ID = 'overview-search';

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
	// from the URL lets a shared/reloaded link resume where "Mehr laden" left
	// off; searching or changing tags resets it back to 1.
	let pageNum = $state(untrack(() => data.page));

	let items = $state<RecipeCardData[]>([]);
	let total = $state(0);
	let allTags = $state<Tag[]>([]);
	let authors = $state<Author[]>([]);

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
	// filter and the favorites switch, written as a sum so a later filter
	// adds its own term here rather than rewriting this into a sum. The
	// tags are counted even though they are set outside the panel the
	// badge sits on - the badge answers "how far is the list narrowed",
	// which is also what "Alle Filter zurücksetzen" inside the panel
	// clears. Sort is deliberately NOT a term, even though it does have a
	// section in that panel: sorting returns the same recipes in a
	// different order, so counting it would be a lie - see the matching
	// note on FilterPanel's `sort` prop.
	const filterCount = $derived(
		tags.length + (maxMinutes > 0 ? 1 : 0) + (favoritesOnly ? 1 : 0) + (author !== '' ? 1 : 0)
	);

	function syncUrl() {
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

	async function fetchPage(pageToFetch: number, append: boolean) {
		abortController?.abort();
		const controller = new AbortController();
		abortController = controller;

		if (append) {
			loadingMore = true;
		} else {
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

	function setSort(value: Sort) {
		sort = value;
		pageNum = 1;
		void fetchPage(1, false);
	}

	function resetFilters() {
		// Sort is deliberately left alone: it isn't a filter (see the note on
		// filterCount), so "reset filters" doesn't touch it - someone who
		// wants the default order back uses the sort control itself, which is
		// where they set it.
		q = '';
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
		{ value: 'title', label: m.overview_sort_title() }
	];

	function handleSort(value: string) {
		// Select's `value` is a plain string; narrowed back to Sort since
		// sortOptions only ever offers the three known values as options.
		if (isSort(value)) {
			setSort(value);
		}
	}

	/**
	 * Follows a URL this page did not write itself: the shell's "Rezepte"
	 * links point back at `/`, the bottom nav's "Suche" adds `?focus=search`,
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
		const next = parseListQuery(page.url);
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
		if (page.url.searchParams.get('focus') === 'search') {
			void focusSearchField();
		}
	}

	/**
	 * Carries out `?focus=search` and drops the param again: it is a one-shot
	 * instruction, and leaving it in the URL would mean every later write had
	 * to preserve it. `syncUrl` rewrites the query string from the list state
	 * alone, which is exactly the URL without it.
	 */
	async function focusSearchField() {
		await tick();
		document.getElementById(SEARCH_FIELD_ID)?.focus();
		syncUrl();
	}

	afterNavigate(followUrl);

	onMount(() => {
		void fetchPage(pageNum, false);
		void listTags()
			.then((result) => (allTags = result))
			.catch(() => {
				// The tag row is a nice-to-have filter; a failure here shouldn't
				// block the recipe grid itself.
			});
		void listAuthors()
			.then((result) => (authors = result))
			.catch(() => {
				// Same as the tags above: without it the panel simply shows no
				// author section.
			});
		return () => abortController?.abort();
	});
</script>

<svelte:head><title>{m.app_name()}</title></svelte:head>

<section class="pt-6 md:pt-10">
	<h1 class="font-display text-display-sm font-medium md:text-display-lg">
		{m.overview_headline()}
	</h1>

	<div class="mt-4 space-y-3 border-b border-border pb-4 md:mt-5">
		<div class="flex items-center gap-3">
			<SearchBar id={SEARCH_FIELD_ID} bind:value={q} onsearch={handleSearch} class="md:max-w-md" />
			<FilterPanel
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
		<TagFilter tags={allTags} active={tags} ontoggle={toggleTag} />
	</div>

	<div class="mt-3">
		<ResultCount count={total} {filtered} />
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
				<Button variant="secondary" disabled title={m.common_coming_soon()}>
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
					<!-- Also disabled while a filter fetch is in flight: "Mehr laden"
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
