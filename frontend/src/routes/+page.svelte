<script lang="ts">
	import { onMount, tick, untrack } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MediaQuery } from 'svelte/reactivity';
	import ArrowDownWideNarrow from '@lucide/svelte/icons/arrow-down-wide-narrow';
	import Columns2 from '@lucide/svelte/icons/columns-2';
	import Grid3x3 from '@lucide/svelte/icons/grid-3x3';
	import LayoutGrid from '@lucide/svelte/icons/layout-grid';
	import Square from '@lucide/svelte/icons/square';
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
		type PantryItem,
		type RecipeCard as RecipeCardData,
		type RecipePage,
		type SearchSection,
		type Tag
	} from '#lib/api/recipes.js';
	import { listPeople, type PersonEntry } from '#lib/api/users.js';
	import Button from '#lib/components/ui/Button.svelte';
	import EmptyState from '#lib/components/ui/EmptyState.svelte';
	import NoteEmptyState from '#lib/components/ui/NoteEmptyState.svelte';
	import firstRecipeArt from '$brand/lockups/rezepte-mascot-first-recipe.webp?url';
	import InstallBanner from '#lib/components/install/InstallBanner.svelte';
	import PushBanner from '#lib/components/install/PushBanner.svelte';
	import UpdateBanner from '#lib/components/install/UpdateBanner.svelte';
	import Select from '#lib/components/ui/Select.svelte';
	import Skeleton from '#lib/components/ui/Skeleton.svelte';
	import ActiveFilters from '#lib/components/recipe/ActiveFilters.svelte';
	import AuthorBand from '#lib/components/recipe/AuthorBand.svelte';
	import CardSizeControl from '#lib/components/recipe/CardSizeControl.svelte';
	import FilterPanel from '#lib/components/recipe/FilterPanel.svelte';
	import RecipeCard from '#lib/components/recipe/RecipeCard.svelte';
	import RecipeGrid from '#lib/components/recipe/RecipeGrid.svelte';
	import ResultCount from '#lib/components/recipe/ResultCount.svelte';
	import SearchBar from '#lib/components/recipe/SearchBar.svelte';
	import TagFilter from '#lib/components/recipe/TagFilter.svelte';
	import TickRow from '#lib/components/recipe/TickRow.svelte';
	import {
		cardSize,
		setCardSize,
		setPhoneColumns,
		type CardSize,
		type PhoneColumns
	} from '#lib/recipe/card-size.svelte.js';
	import { missingAfterTicks } from '#lib/recipe/finds.js';
	import { formatDay } from '#lib/recipe/format.js';
	import { overviewViewState } from '#lib/recipe/overview-state.js';
	import { buildListQuery, isSort, parseListQuery, type Sort } from '#lib/recipe/query.js';
	import { ticksNamed } from '#lib/recipe/ticks.js';
	import { applyRememberedSort, readSort, writeSort } from '#lib/recipe/sort-preference.js';
	import { importDialog } from '#lib/import.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { session } from '#lib/auth.svelte.js';
	import type { PageProps, Snapshot } from './$types';

	let { data }: PageProps = $props();

	// Seeded once from the initial load - after that, this component owns
	// them (searching/toggling tags updates them directly rather than
	// re-deriving from `data`, which would fight with those updates), and
	// `followUrl` below reseeds them when the URL changes for another reason.
	// `untrack` marks that one-time read as intentional.
	let q = $state(untrack(() => data.q));
	let tags = $state<string[]>(untrack(() => data.tags));
	let ingredients = $state<string[]>(untrack(() => data.ingredients));
	/** The ticks, the user's own; a name from the URL is keyed as the server keys it. */
	let have = $state<PantryItem[]>(untrack(() => ticksNamed(data.have)));
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
	// off in the hours around midnight. Swedish writes dates ISO-style.
	const todayIso = today.toLocaleDateString('sv');

	let items = $state<RecipeCardData[]>([]);
	let total = $state(0);
	let lastPage = $state(false);
	/** A search's results counted by section; absent without a search. */
	let sections = $state<RecipePage['sections']>();
	/**
	 * The ingredient search's sections, counted by how many chips a recipe
	 * uses, with the chips they were counted for: the headings name those,
	 * not chips added since.
	 */
	let pantrySections = $state<{ counts: Record<number, number>; chips: string[] }>();
	/**
	 * The tick row's order, what the ranked cards miss: frozen for the
	 * filters it was fetched for (`rowFor`), so a tick, a clear or an undo
	 * never moves a pill.
	 */
	let row = $state.raw<PantryItem[]>([]);
	let rowFor = '';
	/** The card the tick row is focused on; view state, never in the URL. */
	let focusedId = $state<string | null>(null);
	/** The first-use hint, until the first tick or focus of this visit. */
	let hint = $state(true);
	/** The pill under the mouse, marked in every card's missing line. */
	let hovered = $state<string | null>(null);
	/**
	 * Room kept under the results from the first focus until the query
	 * changes: a re-sort or a shorter missing line would otherwise shorten
	 * the page, and the browser would pull the pinned card up with it.
	 */
	let reserve = $state(0);
	/**
	 * The grids hold every card's height and column from a focus until the
	 * next list is shown, so ticking moves no card, and "Done" lets go in
	 * the same step the re-sort glides in (see RecipeGrid).
	 */
	let frozen = $state(false);
	/**
	 * The general row's height when a desktop card was focused: the focused
	 * row keeps at least that, or the grid under it would rise (the hint
	 * goes, the focused row is shorter) at the top of a page that cannot
	 * scroll up to make up for it.
	 */
	let rowHeight = $state(0);
	/** What the polite live region last said: the card a stamp focused. */
	let announcement = $state('');
	let resortTimer: ReturnType<typeof setTimeout> | undefined;
	/** The controller of the list fetch under way when it is a quiet re-sort. */
	let quietController: AbortController | null = null;
	/** The ticks the cards on screen were ranked by, for the pages "Load more" adds. */
	let rankedHave: string[] = [];
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
	let authorsLoaded = $state(false);
	/** Everyone with an account, fetched only for an author /authors does not know. */
	let people = $state<PersonEntry[] | null>(null);
	// A link to someone who has not written a recipe yet (from the people
	// list or a person page): /authors lists only those who have, so the band
	// and the chip would fall back to the bare login name.
	$effect(() => {
		if (!authorsLoaded || author === '' || people !== null) return;
		if (authors.some((entry) => entry.username === author)) return;
		people = [];
		listPeople()
			.then((result) => (people = result))
			.catch(() => {
				// Without it the chip names the login name, as for someone who left.
			});
	});
	/** Narrowed to one known person: the band in their color heads the results. */
	const bandPerson = $derived.by((): Author | undefined => {
		const known = authors.find((entry) => entry.username === author);
		if (known) return known;
		const entry = people?.find((candidate) => candidate.username === author);
		return entry && { ...entry, count: 0 };
	});
	/** The authors plus the band's person when they have no recipes, for the chips. */
	const chipAuthors = $derived(
		bandPerson && !authors.includes(bandPerson) ? [...authors, bandPerson] : authors
	);

	let filterOpen = $state(false);

	let loading = $state(false);
	let loadingMore = $state(false);
	// Where the cards "Load more" brought start, so only they rise into
	// place; a fresh list (a filter, a reload) has none.
	let appendedFrom = $state(Infinity);
	// Set for the first list after an arrival, from a switch or a sign-in
	// (#lib/arrival.ts): every card rises then, on the arrival's own timing
	// (`data-arrival` in app.css).
	let arrival = $state(false);
	let riseTimer: ReturnType<typeof setTimeout> | undefined;
	let showSkeleton = $state(false);

	/**
	 * Back from a recipe, the reader lands where they left the list. The
	 * router restores the scroll position the moment the page mounts, when
	 * the list is not fetched yet and the page too short to hold it, so the
	 * browser clamps it. The snapshot keeps the position with the history
	 * entry, and the first list scrolls back to it once its cards are laid out.
	 */
	let restoreY: number | null = null;
	export const snapshot: Snapshot<number> = {
		capture: () => window.scrollY,
		restore: (y) => (restoreY = y)
	};

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
		q !== '' ||
			tags.length > 0 ||
			ingredients.length > 0 ||
			maxMinutes > 0 ||
			favoritesOnly ||
			author !== ''
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
	const canLoadMore = $derived(viewState.canLoadMore && !lastPage);
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

	function focusFilters(options?: FocusOptions) {
		document.getElementById('overview-filters')?.focus(options);
	}

	// What both rows of active-filter chips get, the phone's and desktop's.
	const filterProps = $derived({
		maxMinutes,
		favoritesOnly,
		author,
		authors: chipAuthors,
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
			ingredients,
			have: have.map((item) => item.name),
			maxMinutes,
			favorites: favoritesOnly,
			author,
			sort,
			page: pageNum
		});
		lastQuery = qs;
		void goto(resolve(qs ? `?${qs}` : '/'), {
			replace: true,
			reset: false
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

	/**
	 * `quiet`, for a re-sort after ticks: the same query, so the cards stay
	 * on screen under no skeleton, and the room kept under them stays.
	 * `anchor`: the card that keeps its place on screen while the others
	 * glide (the one "Done" let go of).
	 */
	async function fetchPage(
		pageToFetch: number,
		append: boolean,
		animate = true,
		{ quiet = false, anchor }: { quiet?: boolean; anchor?: string } = {}
	) {
		abortController?.abort();
		const controller = new AbortController();
		abortController = controller;
		quietController = quiet ? controller : null;
		// "Load more" pages the ranking on screen, so a re-sort the ticks
		// started still comes after it.
		if (!append) clearTimeout(resortTimer);
		const sentHave = append
			? rankedHave
			: ingredients.length > 0
				? have.map((item) => item.name)
				: [];
		// What the row and the section headings are fetched for: every
		// filter that changes the cards, as the row lists what they miss.
		const sent = { q, tags, ingredients, maxMinutes, favoritesOnly, author };

		if (append) {
			loadingMore = true;
		} else if (quiet) {
			loading = true;
		} else {
			reserve = 0;
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
			const fetchOne = (n: number) =>
				listRecipes(
					{
						q: q || undefined,
						tags,
						ingredients,
						// Ticks only count against chips; a page appended counts
						// the ones the cards above it were ranked by.
						have: sentHave,
						maxMinutes,
						favorites: favoritesOnly,
						author: author || undefined,
						sort,
						page: n
					},
					{ signal: controller.signal }
				);
			// A fresh list for page N - a reload or a shared link after "Load
			// more" - shows pages 1 to N, as the list looked when the URL was
			// written, not page N alone. Page 1 first: its total caps N at the
			// last page there is, so `?page=9999` costs one request, not 9999.
			const pages = [await fetchOne(append ? pageToFetch : 1)];
			if (!append) {
				const last = Math.min(pageToFetch, Math.ceil(pages[0].total / pages[0].limit));
				pages.push(
					...(await Promise.all(Array.from({ length: last - 1 }, (_, i) => fetchOne(i + 2))))
				);
			}
			const result = pages[pages.length - 1];
			const fetched = pages.flatMap((p) => p.items);
			const show = () => {
				// A glide hands this over a frame later, by when a newer
				// request may have taken over, or a focus aborted this one.
				if (controller !== abortController || controller.signal.aborted) {
					return;
				}
				arrival = !listed && 'arrival' in document.documentElement.dataset;
				appendedFrom = arrival ? 0 : append ? items.length : Infinity;
				// Once they have risen the cards drop the rise again: kept, it
				// played anew whenever the grid laid them out afresh (a card
				// size change), and the cards vanished and rose a second time.
				// The longest rise, an arrival's eighth card or a "Load more"
				// batch's ninth, is over within a second.
				clearTimeout(riseTimer);
				if (appendedFrom !== Infinity) {
					riseTimer = setTimeout(() => {
						appendedFrom = Infinity;
						arrival = false;
					}, 1200);
				}
				// Pages are cut by offset: a recipe saved since an earlier page
				// moves one onto the next, which would show it twice. It is
				// then one card short of the total, so the last page, not the
				// count, ends "Load more".
				const shown = new Set(append ? items.map((recipe) => recipe.id) : []);
				const fresh = fetched.filter(
					(recipe, at) =>
						!shown.has(recipe.id) && fetched.findIndex((other) => other.id === recipe.id) === at
				);
				items = append ? [...items, ...fresh] : fresh;
				total = result.total;
				lastPage = result.page * result.limit >= result.total;
				sections = result.sections;
				pantrySections = result.pantry?.sections && {
					counts: result.pantry.sections,
					chips: sent.ingredients
				};
				const frozenFor = JSON.stringify(sent);
				if (!append && frozenFor !== rowFor) {
					row = result.pantry?.row ?? [];
					rowFor = frozenFor;
				}
				pageNum = result.page;
				if (!append) {
					frozen = false;
					rankedHave = sentHave;
				}
				// Inside the update, so the glide's picture of the new state
				// already has the skeleton or the empty state where it belongs.
				settle();
				syncUrl();
			};
			// A new list replaces the cards on screen, and the ones that stay
			// glide to their new place; "Load more" only adds below, the
			// search while typing would glide at every pause, and the first
			// list has nothing to glide from.
			if (append || !animate || !listed) {
				show();
				if (restoreY !== null) {
					await tick();
					window.scrollTo(0, restoreY);
					restoreY = null;
				}
			} else {
				const pinned = anchor
					? document.querySelector<HTMLElement>(`main [data-card="${anchor}"]`)
					: null;
				await glide(show, pinned ?? false);
			}
			listed = true;
		} catch (err) {
			if (err instanceof DOMException && err.name === 'AbortError') {
				return;
			}
			frozen = false;
			toast.error(m.overview_load_error());
		} finally {
			settle();
		}

		// Guard against a superseded (aborted) request clearing the loading
		// state of the request that replaced it.
		function settle() {
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
		focusedId = null;
		void fetchPage(1, false, false);
	}

	/** A filter, the chips or the order changed: back to the first page, and
	 * the row lets go of its card. */
	function refilter() {
		pageNum = 1;
		focusedId = null;
		void fetchPage(1, false);
	}

	function toggleTag(name: string) {
		tags = tags.includes(name) ? tags.filter((t) => t !== name) : [...tags, name];
		refilter();
	}

	/**
	 * The chips changed: the row lets go of its card. The first chip ranks
	 * by how much of them a recipe uses, over a remembered order, without
	 * forgetting it; the last one gives the remembered order back.
	 */
	function setIngredients(next: string[]) {
		const remembered = readSort();
		if (ingredients.length === 0 && next.length > 0 && remembered !== null && sort === remembered) {
			sort = 'updated';
		} else if (ingredients.length > 0 && next.length === 0 && sort === 'updated') {
			sort = remembered ?? 'updated';
		}
		ingredients = next;
		refilter();
	}

	// A ticked suggestion becomes a chip and its own tick goes, so the URL
	// never names one ingredient in both lists. A broader tick that covers
	// it stays: picking "Rotweinessig" keeps "Essig" ticked.
	function addIngredient(name: string, key = name.toLocaleLowerCase()) {
		const lower = name.toLocaleLowerCase();
		if (ingredients.some((n) => n.toLocaleLowerCase() === lower)) {
			return;
		}
		have = have.filter((item) => item.key !== key && item.name.toLocaleLowerCase() !== lower);
		setIngredients([...ingredients, name]);
	}

	function removeIngredient(name: string) {
		setIngredients(ingredients.filter((n) => n !== name));
	}

	/**
	 * A tick shows on every card at once; the order follows from the
	 * server 0.9 s after the last one, or on "Done" while a card is focused,
	 * which stays where the reader looks.
	 */
	function setTicks(next: PantryItem[]) {
		have = next;
		hint = false;
		syncUrl();
		clearTimeout(resortTimer);
		// A re-sort under way for the last ticks would glide under this one.
		if (abortController !== null && abortController === quietController) {
			abortController.abort();
		}
		// `focusedCard`, not `focusedId`: a focus whose card left the list
		// holds nothing still, and the re-sort would never come.
		if (focusedCard === undefined) {
			resortTimer = setTimeout(() => void fetchPage(pageNum, false, true, { quiet: true }), 900);
		}
	}

	function toggleTick(item: PantryItem) {
		setTicks(
			have.some((t) => t.key === item.key)
				? have.filter((t) => t.key !== item.key)
				: [...have, item]
		);
	}

	function clearTicks() {
		const before = have;
		setTicks([]);
		toast(before.length === 1 ? m.ticks_cleared_one() : m.ticks_cleared({ count: before.length }), {
			action: { label: m.ticks_undo(), onClick: () => setTicks(before) }
		});
	}

	/** Where a card's top is on the page, which a scroll does not change. */
	function cardY(id: string): number | null {
		const el = document.querySelector(`main [data-card="${id}"]`);
		return el ? el.getBoundingClientRect().top + window.scrollY : null;
	}

	/**
	 * Focuses the tick row on a card, or lets go of it (a second tap, or
	 * "Done"). The card keeps its place on screen as the row above it
	 * changes; a focused card under the sticky row comes out below it, and
	 * one let go of stays put while the others re-sort and glide.
	 */
	async function focusCard(id: string | null) {
		const was = focusedId;
		const pinned = id ?? was;
		if (pinned === null) return;
		const before = cardY(pinned);
		if (id !== null) {
			// The card holds still from here: a re-sort the last ticks
			// started waits for "Done", which fetches anyway.
			clearTimeout(resortTimer);
			if (abortController !== null && abortController === quietController) {
				abortController.abort();
			}
			if (results) reserve = Math.max(reserve, results.offsetHeight + 320);
			frozen = true;
			if (was === null) rowHeight = phone.current ? 0 : (tickRow?.offsetHeight ?? 0);
		} else {
			rowHeight = 0;
		}
		focusedId = id;
		hint = false;
		await tick();
		const after = cardY(pinned);
		if (before !== null && after !== null) {
			window.scrollBy(0, after - before);
		}
		if (id !== null) {
			// The keyboard follows the row: its first pill, or "Done" when
			// nothing is missing, and a screen reader hears which card it is.
			const target = focusedCard?.pantry && focusTarget();
			target?.focus({ preventScroll: true });
			if (focusedCard?.pantry) {
				const { title } = focusedCard;
				const count = missingAfterTicks(focusedCard.pantry.missing, have).length;
				announcement =
					count === 0
						? m.ticks_focus_all_there({ title })
						: count === 1
							? m.ticks_focus_announce_one({ title })
							: m.ticks_focus_announce({ title, count });
			}
			const card = document.querySelector(`main [data-card="${id}"]`);
			const rowBottom = tickRow?.getBoundingClientRect().bottom ?? 0;
			const gap = (card?.getBoundingClientRect().top ?? Infinity) - (rowBottom + 12);
			// On a phone the card always comes to sit 12px under the sticky
			// row; a desktop only lifts one the row would cover.
			if (gap < 0 || (gap !== Infinity && phone.current)) {
				window.scrollBy(0, gap);
			}
		} else if (was !== null) {
			// Let go from the row, the keyboard goes back to the stamp it came from.
			if (tickRow?.contains(document.activeElement)) {
				document
					.querySelector<HTMLElement>(`main [data-card="${was}"] [data-stamp]`)
					?.focus({ preventScroll: true });
			}
			void fetchPage(pageNum, false, true, { quiet: true, anchor: was });
		}
	}

	function focusTarget(): HTMLElement | null {
		return (
			tickRow?.querySelector<HTMLElement>('[role="group"] button') ??
			[...(tickRow?.querySelectorAll<HTMLElement>('button') ?? [])].at(-1) ??
			null
		);
	}

	/** Escape in the focused row is "Done". */
	function handleKeydown(event: KeyboardEvent) {
		if (event.key !== 'Escape' || event.defaultPrevented || focusedId === null) return;
		const id = focusedId;
		void focusCard(null);
		document
			.querySelector<HTMLElement>(`main [data-card="${id}"] [data-stamp]`)
			?.focus({ preventScroll: true });
	}

	let tickRow = $state<HTMLElement>();
	const phone = new MediaQuery('max-width: 767.98px');

	// While a card is focused it stays where it is on screen: whatever moves
	// it on the page - a missing line above it shrinking as items are ticked,
	// the masonry placing it anew - the window scrolls by as much. Its place
	// on the page is read every frame and after every move the masonry
	// writes, so the correction lands before the browser paints.
	$effect(() => {
		const id = focusedId;
		if (id === null) return;
		let y = cardY(id);
		const hold = () => {
			const now = cardY(id);
			if (now !== null && y !== null && now !== y) {
				window.scrollBy(0, now - y);
			}
			y = now;
		};
		let frame = requestAnimationFrame(function loop() {
			hold();
			frame = requestAnimationFrame(loop);
		});
		const moves = new MutationObserver(hold);
		const card = document.querySelector(`main [data-card="${id}"]`);
		if (card) moves.observe(card, { attributes: true, attributeFilter: ['style'] });
		return () => {
			cancelAnimationFrame(frame);
			moves.disconnect();
		};
	});

	function setMaxMinutes(value: number) {
		maxMinutes = value;
		refilter();
	}

	function setFavoritesOnly(value: boolean) {
		favoritesOnly = value;
		refilter();
	}

	function setAuthor(value: string) {
		author = value;
		refilter();
	}

	// Only the sort control and the phone's sort chip come through here, so
	// this is where a choice of order is remembered; a link that names one
	// is applied by `load` and `followUrl` without being stored.
	function setSort(value: Sort, remember = true) {
		if (remember) {
			writeSort(value);
		}
		sort = value;
		refilter();
	}

	// Sort is deliberately left alone: it isn't a filter (see the note on
	// filterCount), so "reset filters" doesn't touch it - someone who wants
	// the default order back uses the sort control itself, which is where
	// they set it.
	// The chips stay too: they narrow the list as well, but they are the
	// search itself, not a filter on it.
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
		refilter();
	}

	function loadMore() {
		void fetchPage(pageNum + 1, true);
	}

	// With a search text or chips the default order puts the best hits first (most
	// recently changed only breaks ties), so it is named for that then.
	const defaultSortLabel = $derived(
		q.trim() || ingredients.length > 0 ? m.overview_sort_relevance() : m.overview_sort_updated()
	);
	const sortOptions: { value: Sort; label: string }[] = $derived([
		{ value: 'updated', label: defaultSortLabel },
		{ value: 'created', label: m.overview_sort_created() },
		{ value: 'title', label: m.overview_sort_title() },
		{ value: 'tasty', label: m.overview_sort_tasty() }
	]);

	const sectionLabels: Record<SearchSection, () => string> = {
		title: m.overview_section_title,
		tags: m.overview_section_tags,
		ingredients: m.overview_section_ingredients,
		description: m.overview_section_description
	};
	// A search in the best-match order comes grouped by where each hit was
	// found. The server orders by section first, so a section's cards come
	// in one run; a card joins its section's group even when it does not
	// (a recipe saved between two pages shifts the offset), so a section
	// never shows twice. `at` keeps each card's place in the whole list for
	// its rise. Only `sections` decides, never `sort`: the order changes on
	// the click, the cards only when the new list arrives. The server
	// sends sections only with the best-match order.
	// The ingredient search groups the same way, by how many chips a card
	// uses, most first, and leads over the text's sections.
	const groups = $derived.by(() => {
		const pantry = pantrySections;
		if (!pantry && !sections) return null;
		const out: {
			key: string;
			label: string;
			count: number;
			cards: { recipe: RecipeCardData; at: number }[];
		}[] = [];
		items.forEach((recipe, at) => {
			const key = pantry
				? `used-${recipe.pantry?.chips ?? 0}`
				: (recipe.match?.section ?? 'description');
			let group = out.find((g) => g.key === key);
			if (!group) {
				let label: string;
				let count: number | undefined;
				if (pantry) {
					const used = recipe.pantry?.chips ?? 0;
					const n = pantry.chips.length;
					label =
						n === 1
							? m.overview_section_one({ name: pantry.chips[0] })
							: used === n
								? m.overview_section_all({ n })
								: m.overview_section_some({ k: used, n });
					count = pantry.counts[used];
				} else {
					const section = key as SearchSection;
					label = sectionLabels[section]();
					count = sections?.[section];
				}
				out.push((group = { key, label, count: count ?? 0, cards: [] }));
			}
			group.cards.push({ recipe, at });
		});
		for (const group of out) group.count ||= group.cards.length;
		return out;
	});

	const showTicks = $derived(ingredients.length > 0 && items.length > 0);
	const focusedCard = $derived(
		focusedId === null ? undefined : items.find((recipe) => recipe.id === focusedId)
	);
	// The row holds the card's missing items as the list last counted them,
	// ticked ones included, so a tick never takes a pill away under the finger.
	const focus = $derived(
		focusedCard?.pantry ? { title: focusedCard.title, missing: focusedCard.pantry.missing } : null
	);

	const sizeOptions = [
		{ value: 'small' as CardSize, label: m.overview_card_size_small(), icon: Grid3x3 },
		{ value: 'medium' as CardSize, label: m.overview_card_size_medium(), icon: LayoutGrid },
		{ value: 'large' as CardSize, label: m.overview_card_size_large(), icon: Square }
	];
	const columnOptions = [
		{ value: 2 as PhoneColumns, label: m.overview_columns_two(), icon: Columns2 },
		{ value: 1 as PhoneColumns, label: m.overview_columns_one(), icon: Square }
	];

	// Whether a list has been shown yet: the first one arrives without a
	// glide, as nothing moves on page load.
	let listed = false;
	// Set while a glide runs: a second one started in it would cut it off.
	let gliding = false;
	let results: HTMLElement | undefined;

	/**
	 * Moves the grid to its next state - a new card size, or a new list for
	 * a filter or an order - and the cards on screen glide from their old
	 * place and size to their new one. The grid lays itself out
	 * synchronously (see RecipeGrid), so once Svelte's update settles the
	 * new layout is there to measure: the view transition takes a picture
	 * of each named card before and after and morphs between them. A card
	 * is named by its recipe, so one that stays in a new list finds itself
	 * again, one that joins fades in at its place and one that left fades
	 * out at its old one; the empty state is named the same way and crosses
	 * over with the cards it replaces or makes way for. Only what is on
	 * screen is named: a picture of every card in a long list held the
	 * first frame up for cards nobody sees move. The rest of
	 * the page is not crossed over (`data-card-transition` in app.css).
	 *
	 * `keepPlace`, for a size change: the first card that starts on screen
	 * stays where it was - every card moves when the columns change, and a
	 * reader far down the list would otherwise lose their place. A new list
	 * holds other cards, so there is no place to keep.
	 *
	 * Reduced motion, a browser without view transitions and a transition
	 * already under way (a page change, a theme change, an earlier glide)
	 * get the new state at once.
	 */
	async function glide(change: () => void, keepPlace: boolean | HTMLElement = false) {
		const root = document.documentElement;
		const shown = () => [
			...document.querySelectorAll<HTMLElement>('main [data-card], main [data-empty]')
		];
		const onScreen = (el: HTMLElement) => {
			const box = el.getBoundingClientRect();
			return box.bottom > 0 && box.top < window.innerHeight;
		};
		const name = (el: HTMLElement) => {
			const card = el.dataset.card;
			el.style.setProperty('view-transition-name', card ? `card-${card}` : 'overview-empty');
			el.style.setProperty('view-transition-class', 'card');
		};
		const unname = (el: HTMLElement) => {
			el.style.removeProperty('view-transition-name');
			el.style.removeProperty('view-transition-class');
		};
		const before = shown();
		// A card cut off at the top is one the reader has already passed.
		const anchor =
			keepPlace instanceof HTMLElement
				? keepPlace
				: keepPlace
					? before
							.filter((el) => el.dataset.card && el.getBoundingClientRect().top >= 0)
							.sort((a, b) => a.getBoundingClientRect().top - b.getBoundingClientRect().top)[0]
					: undefined;
		const top = anchor?.getBoundingClientRect().top;
		const animate =
			'startViewTransition' in document &&
			!prefersReducedMotion.current &&
			!gliding &&
			!('pageTransition' in root.dataset) &&
			!('themeTransition' in root.dataset);
		const named = animate ? before.filter(onScreen) : [];
		const update = async () => {
			change();
			await tick();
			if (anchor && top !== undefined && window.scrollY > 0) {
				window.scrollBy(0, anchor.getBoundingClientRect().top - top);
			}
			if (!animate) {
				return;
			}
			for (const el of shown()) {
				if (!named.includes(el) && onScreen(el)) {
					named.push(el);
					name(el);
				}
			}
			// One that glided off screen faded as it went: the browser's
			// picture of its new place below the fold came out empty. It
			// fades out where it was instead, as one that left the list does.
			for (const el of named) {
				if (el.isConnected && !onScreen(el)) {
					unname(el);
				}
			}
		};
		if (!animate) {
			await update();
			return;
		}
		gliding = true;
		// Scoped to the results where the browser can: a transition takes
		// every click inside its scope for its length (the hit lands on the
		// scope, and `pointer-events: none` on the pseudo-elements does not
		// pass it on), and over the whole document a second tag picked right
		// after the first was lost.
		// The fallback is `document`'s, not `<html>`'s: only Chromium gives
		// elements a `startViewTransition`, and Safari threw on the root's.
		// A size change keeps its place over the whole document too: a scoped
		// transition holds its scope at the old height until it ends, so the
		// page could not scroll past the old bottom, and a phone going to one
		// card per row (the list twice as tall) lost the reader's card.
		// So does a long list: Chromium's picture of a scope much taller than
		// the window drew stretched for its first frame, and the whole grid
		// jumped down and back (seen from about twice the window's height).
		const scoped =
			keepPlace !== true &&
			results &&
			'startViewTransition' in results &&
			results.offsetHeight <= 1.5 * window.innerHeight
				? results
				: null;
		const marked = scoped ?? root;
		marked.dataset.cardTransition = '';
		named.forEach(name);
		// Over the whole document the cards' pictures lie above the page, and
		// so above a dialog or menu still open on it - the phone's filter
		// panel changes the order and stays open - and glided across it. A
		// named layer's picture is lifted over theirs (`.layer` in app.css):
		// the order of the pictures does not follow the document's.
		// Scoped, the cards stay inside the results and need none of this.
		const layers = scoped
			? []
			: [
					...document.querySelectorAll<HTMLElement>(
						'[data-dialog-overlay], [role="dialog"], [role="listbox"], [role="menu"]'
					)
				].filter((el) => el.getClientRects().length > 0);
		layers.forEach((el, i) => {
			el.style.setProperty('view-transition-name', `layer-${i}`);
			el.style.setProperty('view-transition-class', 'layer');
		});
		const transition = scoped
			? (scoped as unknown as Pick<Document, 'startViewTransition'>).startViewTransition(update)
			: document.startViewTransition(update);
		// A transition the browser aborts (its layout check failed while the
		// list changed) rejects `ready` as well as `finished`; the list still
		// updates, only without the glide, so neither is an error.
		transition.ready.catch(() => {});
		await transition.finished.catch(() => {});
		gliding = false;
		delete marked.dataset.cardTransition;
		named.forEach(unname);
		layers.forEach((el) => {
			el.style.removeProperty('view-transition-name');
			el.style.removeProperty('view-transition-class');
		});
	}

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
	 * `buildListQuery` normalizes what it is handed (see `#lib/recipe/query`),
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
			ingredients = next.ingredients;
			have = ticksNamed(next.have);
			focusedId = null;
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
			.then((result) => {
				authors = result;
				authorsLoaded = true;
			})
			.catch(() => {
				// Without it the panel simply shows no author section.
			});
		return () => {
			abortController?.abort();
			suggestedTags.abort();
			rowTags.abort();
			clearTimeout(riseTimer);
			clearTimeout(resortTimer);
		};
	});
</script>

<svelte:head><title>{m.app_name()}</title></svelte:head>
<svelte:window onkeydown={handleKeydown} />

<section class="pt-5 md:pt-8">
	<InstallBanner />
	<PushBanner />
	<UpdateBanner />
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
		<!-- On a phone the cards-per-row switch takes the date's place: a
		     320px line has no room for both, and the search row below none
		     for a third control. -->
		<span aria-hidden="true" class="hidden md:inline">·</span>
		<time datetime={todayIso} class="hidden md:inline">{formatDay(today)}</time>
		<div class="self-center md:hidden">
			<CardSizeControl
				small
				value={cardSize.phone}
				options={columnOptions}
				label={m.overview_columns()}
				onchange={(columns) => glide(() => setPhoneColumns(columns), true)}
			/>
		</div>
	</div>

	<h1 class="font-display text-display-sm font-medium md:text-display-lg">
		{m.overview_headline()}
	</h1>

	<!-- The same dotted leader closes the filters off from the grid. -->
	<!-- `gap`, not `space-y`: the phone's chip row stays in the DOM on desktop,
	     hidden, and as the last child it gave the tag row above a margin. -->
	<div
		class="mt-4 flex flex-col gap-3 border-b-2 border-dotted border-handle pb-4 md:mt-5 {showTicks
			? 'max-md:border-0 max-md:pb-0'
			: ''}"
	>
		<div class="relative flex items-center gap-3">
			<SearchBar
				bind:value={q}
				onsearch={handleSearch}
				tags={tagOptions}
				active={tags}
				ontoggle={toggleTag}
				{ingredients}
				{have}
				onaddingredient={addIngredient}
				onremoveingredient={removeIngredient}
				class={ingredients.length > 0 ? '' : 'md:max-w-md'}
			/>
			<FilterPanel
				bind:open={filterOpen}
				{maxMinutes}
				{favoritesOnly}
				{author}
				{authors}
				{sort}
				{sortOptions}
				{filterCount}
				onmaxminutes={setMaxMinutes}
				onfavorites={setFavoritesOnly}
				onauthor={setAuthor}
				onsort={setSort}
				onreset={resetFilters}
				compact={ingredients.length > 0}
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
			<div class="hidden md:block">
				<CardSizeControl
					value={cardSize.desktop}
					options={sizeOptions}
					label={m.overview_card_size()}
					onchange={(size) => glide(() => setCardSize(size), true)}
				/>
			</div>
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
		<!-- On a phone the author band says who, and drops them with its ×. -->
		{#if filterCount > (bandPerson ? 1 : 0) || sort !== 'updated'}
			<ActiveFilters
				{...filterProps}
				author={bandPerson ? '' : author}
				sortLabel={sort === 'updated'
					? ''
					: (sortOptions.find((option) => option.value === sort)?.label ?? '')}
				{defaultSortLabel}
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

	<!-- The tick row, under the search: on a phone it always sticks to the
	     top and takes over the dotted leader; on a desktop it sticks while
	     a card is focused, as a sheet of its own over the cards. Dark mode
	     has no card shadow, so there the sheet is a raised surface with an
	     edge. -->
	{#if showTicks}
		<div
			bind:this={tickRow}
			data-tick-row
			style:min-height={focus && rowHeight ? `${rowHeight}px` : undefined}
			class="sticky top-0 z-20 -mx-5 mt-1.5 border-b-2 border-dotted bg-background px-5 pt-1.5 pb-2 md:mt-4.5 md:border-0 {focus
				? 'border-primary md:-mx-4 md:rounded-b-2xl md:px-4 md:py-2.5 md:shadow-card dark:md:border dark:md:border-t-0 dark:md:border-border dark:md:bg-surface-elevated'
				: 'border-handle md:static md:mx-0 md:p-0'}"
		>
			<p aria-live="polite" class="sr-only">{announcement}</p>
			<TickRow
				{row}
				ticked={have}
				{focus}
				ontoggle={toggleTick}
				onclear={clearTicks}
				ondone={() => focusCard(null)}
				{hint}
				onhover={(item) => (hovered = item?.key ?? null)}
			/>
		</div>
	{/if}

	<div
		bind:this={results}
		data-arrival={arrival || undefined}
		style:min-height={reserve ? `${reserve}px` : undefined}
		class="mt-7 md:mt-9"
	>
		{#if bandPerson}
			<AuthorBand
				person={bandPerson}
				self={bandPerson.id === session.user?.id}
				countShown={bandPerson.count !== total}
				onclear={() => {
					// The × goes with the band, and the focus with it to "Filters",
					// for a tap too: left on <body>, a screen reader's next swipe
					// started over at the top of the page.
					setAuthor('');
					focusFilters({ preventScroll: true });
				}}
			/>
		{/if}
		{#if showSkeleton}
			<RecipeGrid>
				{#each Array.from({ length: 8 }, (_, i) => i) as i (i)}
					<!-- `desk:`, like the card it stands in for (see RecipeGrid). -->
					<div class="rounded-3xl bg-surface p-1.5 shadow-card desk:rounded-4xl desk:p-2">
						<Skeleton class="aspect-[4/3] rounded-xl desk:rounded-2xl" />
						<div class="space-y-2 px-2.5 pt-2.5 pb-4">
							<Skeleton class="h-3.5 w-3/4 rounded-sm" />
							<Skeleton class="h-2.5 w-1/2 rounded-sm" />
						</div>
					</div>
				{/each}
			</RecipeGrid>
		{:else if isEmpty}
			<NoteEmptyState
				art={firstRecipeArt}
				title={m.overview_empty_title()}
				steps={[
					[m.overview_empty_step_write(), m.overview_empty_step_write_hint()],
					[m.overview_empty_step_import(), m.overview_empty_step_import_hint()],
					[m.overview_empty_step_cook(), m.overview_empty_step_cook_hint()]
				]}
			>
				<Button
					variant="secondary"
					size="lg"
					onclick={() => (importDialog.open = true)}
					class="h-12! whitespace-nowrap max-md:px-3"
				>
					{m.overview_import()}
				</Button>
				<Button
					variant="primary"
					size="lg"
					href={resolve('/recipes/new')}
					class="group h-12! whitespace-nowrap shadow-cta max-md:px-3"
				>
					<Plus
						aria-hidden="true"
						strokeWidth={2.5}
						class="size-5 transition-transform duration-300 ease-spring group-hover:rotate-90 motion-reduce:transition-none"
					/>
					{m.overview_new_recipe()}
				</Button>
			</NoteEmptyState>
		{:else if isNoResults}
			<!-- Only a typed term gets quoted back. Tags, time and favorites
			     narrow the list just as well, but naming them in the same
			     sentence would quote something nobody typed - and with none of
			     them typed at all, the sentence used to close around an empty
			     pair of quotation marks. Which tags are on is legible anyway:
			     their chips sit above the grid with the active ones marked. -->
			{#if ingredients.length > 0}
				<!-- Chips are picked from the recipes there are, so mostly a filter
				     or the text empties the grid: resetting those leads and the
				     chips stay, dropping the chips comes second. A chip that
				     matches nothing (a hand-written link) leaves only the second. -->
				{@const narrowed = filterCount > 0 || q !== ''}
				<EmptyState
					title={m.overview_no_results_title()}
					text={q
						? m.empty_pantry_text({ names: ingredients.join(', '), query: q })
						: filterCount > 0
							? m.empty_pantry({ names: ingredients.join(', ') })
							: m.empty_pantry_none({ names: ingredients.join(', ') })}
				>
					{#if narrowed}
						<Button variant="primary" onclick={resetFilters}>{m.overview_reset_filters()}</Button>
					{/if}
					<Button variant={narrowed ? 'secondary' : 'primary'} onclick={() => setIngredients([])}>
						{m.empty_remove_ingredients()}
					</Button>
				</EmptyState>
			{:else}
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
							? resolve(`recipes/new?title=${encodeURIComponent(q)}`)
							: resolve('/recipes/new')}
					>
						{q ? m.overview_create_from_search() : m.overview_new_recipe()}
					</Button>
				</EmptyState>
			{/if}
		{:else}
			{#snippet card(recipe: RecipeCardData, at: number)}
				<RecipeCard
					{recipe}
					rise={at >= appendedFrom ? at - appendedFrom : null}
					ticked={have}
					focused={recipe.id === focusedId}
					onfocuspantry={() => focusCard(recipe.id === focusedId ? null : recipe.id)}
					highlightKey={hovered}
				/>
			{/snippet}
			{#if groups}
				{#each groups as group (group.key)}
					<!-- A section's title is quiet, in the running head's italic: it
					     orders the hits without outweighing the h1 or the cards'
					     titles. In the text color, not the accent, which the text
					     cards' titles already wear and which misses AA on the page's
					     background at this size. The count is all of the section's
					     hits, not just the ones loaded. -->
					<section aria-labelledby="section-{group.key}" class="mt-8 first:mt-0 md:mt-10">
						<h2
							id="section-{group.key}"
							class="mb-2.5 flex flex-wrap items-baseline gap-x-2 font-display text-body-lg font-medium italic md:mb-3"
						>
							<span>{group.label}</span>
							<span
								class="font-sans text-caption font-medium text-text-muted not-italic tabular-nums"
								>· {group.count === 1
									? m.overview_section_count_one()
									: m.overview_section_count({ count: group.count })}</span
							>
						</h2>
						<RecipeGrid {frozen}>
							{#each group.cards as { recipe, at } (recipe.id)}
								{@render card(recipe, at)}
							{/each}
						</RecipeGrid>
					</section>
				{/each}
			{:else}
				<RecipeGrid {frozen}>
					{#each items as recipe, i (recipe.id)}
						{@render card(recipe, i)}
					{/each}
				</RecipeGrid>
			{/if}
			<!-- The button's row stays once more were loaded, empty after the
			     last page: taking it away would shorten the page by its height
			     just as the last cards arrive, and a reader at the bottom would
			     see the browser pull the whole list down by that much. -->
			{#if canLoadMore || pageNum > 1}
				<div class="mt-8 flex h-10 justify-center">
					{#if canLoadMore}
						<!-- Also disabled while a filter fetch is in flight: "Load more"
						     would abort it and append the new filter's page N+1 to the
						     items the old filter left behind. -->
						<Button variant="secondary" onclick={loadMore} disabled={loading || loadingMore}>
							{m.overview_load_more()}
						</Button>
					{/if}
				</div>
			{/if}
		{/if}
	</div>
</section>
