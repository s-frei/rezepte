<script lang="ts">
	import { Dialog } from 'bits-ui';
	import Filter from '@lucide/svelte/icons/filter';
	import { fade, fly, scale } from 'svelte/transition';
	import type { TransitionConfig } from 'svelte/transition';
	import Button from '$lib/components/ui/Button.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import TagChip from '$lib/components/ui/TagChip.svelte';
	import Slider from '$lib/components/ui/Slider.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import { formatMinutes } from '$lib/recipe/format';
	import { isSort, type Sort } from '$lib/recipe/query';
	import type { Author } from '$lib/api/recipes';
	import { userColorClasses } from '$lib/user/color';
	import { TIME_STOPS, timeStopIndex, timeStopMinutes } from '$lib/recipe/time-filter';
	import { m } from '$lib/paraglide/messages';

	// The filter dialog itself - centered on desktop, a bottom sheet on
	// phones. One section per filter kind ("Zeit", "Favoriten" and
	// "Sortierung"): each is its own sibling `<section>` rather than a
	// rebuild of this shell.
	//
	// Tags are deliberately not among them. They had a section here once,
	// listing the same tags the chip row on the page already shows - two
	// places to set one filter, and the tag list was long enough to push
	// every other section out of sight. The chip row is the one place tags
	// are filtered from (see TagFilter.svelte); what is left here is what
	// has nowhere else to live.
	//
	// Bits UI `Dialog` rather than
	// `Popover`: Popover positions its content with Floating UI, which
	// writes inline `position`/`transform` that Tailwind classes can't
	// reliably override, so the phone sheet would fight the library.
	// `Dialog.Content`'s classes are ours to place, exactly like
	// `BaseDialog` and `CommandPalette`.
	//
	// The sort section only renders here on phones (`md:hidden` below): on
	// desktop the same control sits in row 1 of the page head, next to this
	// panel's own trigger, where there is room for it outside the panel.
	let {
		maxMinutes,
		favoritesOnly,
		author,
		authors,
		sort,
		filterCount,
		onmaxminutes,
		onfavorites,
		onauthor,
		onsort,
		onreset
	}: {
		/** Currently selected maximum total time in minutes, 0 = off -
		 * scoped to this section, unlike `filterCount`. */
		maxMinutes: number;
		/** Whether the "Nur Favoriten" switch is on - scoped to this
		 * section, like `maxMinutes`. */
		favoritesOnly: boolean;
		/** Username the list is narrowed to, `''` = off - scoped to this
		 * section, like `maxMinutes`. */
		author: string;
		/** Everyone who has written a recipe, with their counts. The section
		 * hides itself below two of them: a household where one person writes
		 * everything has nothing to filter. */
		authors: Author[];
		/** Currently selected result order - scoped to this section, like
		 * `maxMinutes`. Not counted into `filterCount`: sorting reorders
		 * the grid rather than narrowing it, so it never turns "no recipes
		 * match" into "no recipes at all" the way a real filter would. */
		sort: Sort;
		/** Total active filters across the whole overview - the tag chips
		 * outside this panel included, since the badge answers "how far is
		 * the list narrowed", not "what is behind this button", and
		 * "Alle Filter zurücksetzen" below clears exactly what it counts.
		 * The page owns that state, this panel owns none of it. */
		filterCount: number;
		onmaxminutes: (value: number) => void;
		onfavorites: (value: boolean) => void;
		onauthor: (value: string) => void;
		onsort: (value: Sort) => void;
		onreset: () => void;
	} = $props();

	const uid = $props.id();
	const timeHeadingId = `${uid}-time-heading`;
	const favoritesHeadingId = `${uid}-favorites-heading`;
	const authorHeadingId = `${uid}-author-heading`;

	const sortHeadingId = `${uid}-sort-heading`;

	const sortOptions: { value: Sort; label: string }[] = [
		{ value: 'updated', label: m.overview_sort_updated() },
		{ value: 'created', label: m.overview_sort_created() },
		{ value: 'title', label: m.overview_sort_title() }
	];

	function handleSort(value: string) {
		// Select's `value` is a plain string (any Bits UI item value fits);
		// narrowed back to Sort since sortOptions only ever offers the three
		// known values as options, so this always succeeds in practice.
		if (isSort(value)) {
			onsort(value);
		}
	}

	// Open state is the panel's own: nothing outside it opens the panel any
	// more - the tag chip row used to, back when the hidden tags lived in
	// here - so the trigger and the "Fertig" button are the only two things
	// that move it.
	let open = $state(false);

	// Where the thumb sits, which is not the same thing as the filter that is
	// running: the label has to follow the drag from the first step, while
	// the grid waits for the release (see `Slider`'s `oncommit`). A writable
	// `$derived` is exactly that shape - the drag assigns to it, and any
	// change to the bound the page holds ("Alle Filter zurücksetzen", the
	// back button, a shared link) recomputes it and takes the thumb back,
	// so it never rests somewhere the grid is not.
	let timeIndex = $derived(timeStopIndex(maxMinutes));

	const timeMinutes = $derived(timeStopMinutes(timeIndex));
	const timeLabel = $derived(
		timeMinutes === 0
			? m.overview_filter_time_any()
			: m.overview_filter_time_upto({ duration: formatMinutes(timeMinutes) })
	);

	const PHONE_QUERY = '(max-width: 767.98px)';

	/**
	 * Scales in place as a centered dialog on desktop, slides up as a sheet
	 * on phones - matching the design system's "Dialog: fade + scale" and
	 * "Sheet: Dialog + slide" rows respectively. A `transition:` directive
	 * only ever names one function, so this picks between the two built-ins
	 * itself; `matchMedia` is read when the transition actually runs (each
	 * mount/unmount), never cached, so a resize between opens can't go stale.
	 */
	function panelTransition(node: HTMLElement): TransitionConfig {
		return window.matchMedia(PHONE_QUERY).matches
			? fly(node, { y: 200, duration: 250 })
			: scale(node, { duration: 200, start: 0.95 });
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Trigger
		class="inline-flex h-11 shrink-0 items-center gap-2 rounded-md border border-border bg-surface px-3.5 text-body-sm font-medium transition hover:brightness-[0.98]"
	>
		<Filter class="size-4" aria-hidden="true" />
		{m.overview_filter()}
		{#if filterCount > 0}
			<span
				class="inline-flex size-5 items-center justify-center rounded-pill bg-inverse text-micro font-semibold text-inverse-foreground"
			>
				{filterCount}
			</span>
		{/if}
	</Dialog.Trigger>
	<Dialog.Portal>
		<Dialog.Overlay forceMount>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed inset-0 z-40 bg-overlay"
						transition:fade={{ duration: 200 }}
					></div>
				{/if}
			{/snippet}
		</Dialog.Overlay>
		<Dialog.Content forceMount preventScroll={false}>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<!-- Desktop: the same centered-dialog placement as `BaseDialog`
					     (fixed, centered by translate, `rounded-3xl`, `shadow-dialog`).
					     Phones (`max-md:`): anchored to the bottom edge instead, full
					     width, square bottom corners, `shadow-sheet` - `shadow-dialog`
					     casts downward and would fall off-screen from an edge pinned
					     to the bottom, `shadow-sheet` casts upward instead. The
					     `max-md:` utilities win on the cascade the same way
					     Button.svelte's `max-sm:h-14` overrides its base `h-12`. The
					     header and footer are `shrink-0`; the body below
					     (`min-h-0 flex-1 overflow-y-auto`) is the panel's only
					     scroll container, and with three short sections it never
					     reaches its own `max-h` to scroll at all - it stays as a
					     fallback for a viewport short enough to squeeze even this,
					     a phone in landscape with the on-screen keyboard up. While
					     the tag list was still a section here, it had a bounded
					     height and a scrollbar of its own inside this one, and two
					     nested scrollbars 200px apart is a guessing game about
					     which one a flick will move. -->
					<div
						{...props}
						class="fixed top-1/2 left-1/2 z-50 flex max-h-[min(70dvh,32rem)] w-[min(24rem,calc(100vw-2.5rem))] -translate-x-1/2 -translate-y-1/2 flex-col rounded-3xl bg-surface p-6 shadow-dialog max-md:inset-x-0 max-md:top-auto max-md:bottom-0 max-md:max-h-[85dvh] max-md:w-full max-md:translate-x-0 max-md:translate-y-0 max-md:rounded-b-none max-md:p-5 max-md:pb-8 max-md:shadow-sheet"
						transition:panelTransition
					>
						<Dialog.Title class="shrink-0 font-display text-heading font-medium">
							{m.overview_filter()}
						</Dialog.Title>
						<div class="mt-4 min-h-0 flex-1 overflow-y-auto">
							<section aria-labelledby={timeHeadingId} class="space-y-2">
								<h3 id={timeHeadingId} class="text-caption font-semibold text-text-muted uppercase">
									{m.overview_filter_time()}
								</h3>
								<!-- The bound in words, above the scale that sets it. It is
								     the slider's only label: tick numbers under the track
								     would be nine of them across 336px, and the one that
								     matters is the one the thumb is on. Muted while the
								     filter is off, like the track itself, so "Beliebig"
								     reads as the resting state rather than a choice made. -->
								<p
									class="text-body font-medium {timeMinutes === 0
										? 'text-text-muted'
										: 'text-text'}"
								>
									{timeLabel}
								</p>
								<Slider
									bind:value={timeIndex}
									max={TIME_STOPS.length - 1}
									label={m.overview_filter_time_label()}
									valueText={timeLabel}
									muted={timeMinutes === 0}
									oncommit={(index) => onmaxminutes(timeStopMinutes(index))}
								/>
							</section>
							<section
								aria-labelledby={favoritesHeadingId}
								class="mt-4 space-y-2 border-t border-border pt-3"
							>
								<h3
									id={favoritesHeadingId}
									class="text-caption font-semibold text-text-muted uppercase"
								>
									{m.overview_filter_favorites_heading()}
								</h3>
								<!-- A switch, not a checkbox: this turns one thing on and it
								     takes effect at once, where a checkbox row in this app
								     marks an item in a list. The label leads and the switch
								     sits at the end of the line, which is where a setting's
								     state is looked for. -->
								<label
									class="flex cursor-pointer items-center justify-between gap-3 rounded-md px-1 py-1.5"
								>
									<span class="text-body-sm">{m.overview_filter_favorites()}</span>
									<Switch
										checked={favoritesOnly}
										label={m.overview_filter_favorites()}
										onchange={onfavorites}
									/>
								</label>
							</section>
							<!-- Hidden below two authors: where one person writes
							     everything there is nothing to pick between, and an
							     empty-looking control would only raise the question why.
							     Each chip carries a dot in the person's color, so the
							     filter and its results read as one thing. -->
							{#if authors.length > 1}
								<section
									aria-labelledby={authorHeadingId}
									class="mt-4 space-y-2 border-t border-border pt-3"
								>
									<h3
										id={authorHeadingId}
										class="text-caption font-semibold text-text-muted uppercase"
									>
										{m.overview_filter_author_heading()}
									</h3>
									<div class="flex flex-wrap gap-2">
										{#each authors as person (person.username)}
											<TagChip
												label={person.displayName}
												count={person.count}
												active={author === person.username}
												onclick={() => onauthor(author === person.username ? '' : person.username)}
											>
												{#snippet leading()}
													<span
														aria-hidden="true"
														class="size-3 rounded-full {userColorClasses(person.color)}"
													></span>
												{/snippet}
											</TagChip>
										{/each}
									</div>
								</section>
							{/if}
							<!-- Phones only: on desktop this same control lives in row 1 of
							     the page head instead (see the note on `let { ... }` above). -->
							<section
								aria-labelledby={sortHeadingId}
								class="mt-4 space-y-2 border-t border-border pt-3 md:hidden"
							>
								<h3 id={sortHeadingId} class="text-caption font-semibold text-text-muted uppercase">
									{m.overview_sort_label()}
								</h3>
								<Select
									value={sort}
									options={sortOptions}
									label={m.overview_sort_label()}
									onchange={handleSort}
									class="w-full justify-between bg-surface-elevated text-text"
								/>
							</section>
						</div>
						<!-- Always present: the overlay strip above a phone sheet can be
						     as thin as 15dvh (`max-h-[85dvh]`) and signals nothing
						     tappable, so relying on it alone as the only way out isn't
						     enough. Filters apply live - nothing here is a pending
						     change to confirm - so "Fertig" just closes the panel;
						     "Alle Filter zurücksetzen" only joins it once something is
						     active. Stacked full-width on phones, inline on desktop,
						     to keep the longer reset label from wrapping mid-word. -->
						<div
							class="mt-4 flex shrink-0 justify-end gap-3 border-t border-border pt-3 max-md:flex-col"
						>
							{#if filterCount > 0}
								<Button
									variant="secondary"
									class="whitespace-nowrap max-md:w-full"
									onclick={onreset}
								>
									{m.overview_filter_reset()}
								</Button>
							{/if}
							<Button
								variant="secondary"
								class="whitespace-nowrap max-md:w-full"
								onclick={() => (open = false)}
							>
								{m.overview_filter_done()}
							</Button>
						</div>
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
