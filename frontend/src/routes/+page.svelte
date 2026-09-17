<script lang="ts">
	import { onMount, tick, untrack } from 'svelte';
	import Plus from 'lucide-svelte/icons/plus';
	import { toast } from 'svelte-sonner';
	import { afterNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		listRecipes,
		listTags,
		type RecipeCard as RecipeCardData,
		type Tag
	} from '$lib/api/recipes';
	import Button from '$lib/components/ui/Button.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import RecipeCard from '$lib/components/recipe/RecipeCard.svelte';
	import RecipeGrid from '$lib/components/recipe/RecipeGrid.svelte';
	import SearchBar from '$lib/components/recipe/SearchBar.svelte';
	import TagFilter from '$lib/components/recipe/TagFilter.svelte';
	import { overviewViewState } from '$lib/recipe/overview-state';
	import { buildListQuery, parseListQuery } from '$lib/recipe/query';
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
	// The furthest page fetched for the current filters - the initial value
	// from the URL lets a shared/reloaded link resume where "Mehr laden" left
	// off; searching or changing tags resets it back to 1.
	let pageNum = $state(untrack(() => data.page));

	let items = $state<RecipeCardData[]>([]);
	let total = $state(0);
	let allTags = $state<Tag[]>([]);

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

	// The API only accepts a single `tag` filter (see the API contract in
	// the phase plan). When more than one tag is selected we send the first
	// to the server and filter the rest client-side within whatever page(s)
	// are already loaded - so a second/third tag can under-report matches
	// that exist beyond the loaded pages until "Mehr laden" is used.
	// See the ledger: .superpowers/sdd/2026-09-17-phase-3-recipe-core/progress.md,
	// "Ruling: multi-tag filter".
	const extraTags = $derived(tags.slice(1));
	const visibleItems = $derived(
		extraTags.length === 0
			? items
			: items.filter((item) => extraTags.every((tag) => item.tags.includes(tag)))
	);

	const viewState = $derived(
		overviewViewState({
			loading,
			q,
			tagCount: tags.length,
			itemCount: items.length,
			visibleItemCount: visibleItems.length,
			total
		})
	);
	const isEmpty = $derived(viewState.isEmpty);
	const isNoResults = $derived(viewState.isNoResults);
	const canLoadMore = $derived(viewState.canLoadMore);
	const searchDescription = $derived(q || tags.join(', '));

	function syncUrl() {
		const qs = buildListQuery({ q, tags, page: pageNum });
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
				{ q: q || undefined, tag: tags[0], page: pageToFetch },
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

	function resetFilters() {
		q = '';
		tags = [];
		pageNum = 1;
		void fetchPage(1, false);
	}

	function loadMore() {
		void fetchPage(pageNum + 1, true);
	}

	/**
	 * Follows a URL this page did not write itself: the shell's "Rezepte"
	 * links point back at `/`, the bottom nav's "Suche" adds `?focus=search`,
	 * and the back button or a shared link restores an earlier filter. `load`
	 * re-runs for all of those, but the filter state lives in this component,
	 * so without reseeding it the grid would keep showing the old filter and
	 * the next `syncUrl()` would put it straight back into the URL.
	 *
	 * `buildListQuery` normalises what it is handed (see `$lib/recipe/query`),
	 * so comparing it against the string `syncUrl` wrote recognises the page's
	 * own updates and leaves them alone.
	 */
	function followUrl() {
		const next = parseListQuery(page.url);
		if (buildListQuery(next) !== lastQuery) {
			q = next.q;
			tags = next.tags;
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
		return () => abortController?.abort();
	});
</script>

<section class="pt-6 md:pt-10">
	<h1 class="font-display text-display-sm font-medium md:text-display-lg">
		{m.overview_headline()}
	</h1>

	<div class="mt-5 flex flex-col gap-3 md:mt-6 md:flex-row md:items-center md:gap-4">
		<SearchBar id={SEARCH_FIELD_ID} bind:value={q} onsearch={handleSearch} />
		<TagFilter tags={allTags} active={tags} ontoggle={toggleTag} />
	</div>

	<div class="mt-6 md:mt-8">
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
			<EmptyState
				title={m.overview_no_results_title()}
				text={m.overview_no_results_text({ query: searchDescription })}
			>
				<Button variant="secondary" onclick={resetFilters}>{m.overview_reset_filters()}</Button>
				<!--
					Still disabled: the editor exists, but carrying the search term
					into it needs a prefill param the API contract does not define
					yet, and a button that drops the term would be a lie.
				-->
				<Button variant="accent" disabled title={m.common_coming_soon()}>
					{m.overview_create_from_search()}
				</Button>
			</EmptyState>
		{:else}
			{#if visibleItems.length > 0}
				<RecipeGrid>
					{#each visibleItems as recipe (recipe.id)}
						<RecipeCard {recipe} />
					{/each}
				</RecipeGrid>
			{:else}
				<!--
					More pages exist server-side (canLoadMore below), but the
					client-side multi-tag filter hides everything loaded so far -
					see the multi-tag comment above `extraTags`. A short hint plus
					"Mehr laden" beats the no-results panel here, since a later
					page may still contain a match.
				-->
				<p class="text-body text-text-muted">{m.overview_more_pages_hint()}</p>
			{/if}
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
