<script lang="ts">
	import ArrowDownWideNarrow from '@lucide/svelte/icons/arrow-down-wide-narrow';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import Clock from '@lucide/svelte/icons/clock';
	import Star from '@lucide/svelte/icons/star';
	import TagIcon from '@lucide/svelte/icons/tag';
	import UserRound from '@lucide/svelte/icons/user-round';
	import X from '@lucide/svelte/icons/x';
	import type { Author } from '$lib/api/recipes';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';
	import TagChip from '$lib/components/ui/TagChip.svelte';
	import { formatMinutes } from '$lib/recipe/format';
	import { m } from '$lib/paraglide/messages';

	let {
		maxMinutes,
		favoritesOnly,
		author,
		authors,
		collapsible = false,
		onopen,
		onmaxminutes,
		onfavorites,
		onauthor,
		sortLabel = '',
		onsortreset,
		tags = [],
		ontagremove,
		onreset,
		focusfallback,
		class: className
	}: {
		/** The filter panel's state, as FilterPanel gets it. */
		maxMinutes: number;
		favoritesOnly: boolean;
		author: string;
		/** For the chosen author's display name and color. */
		authors: Author[];
		/** Phones: once the row would need a second line, fold everything in
		 * it, the tags too, into one bar that unfolds it again. */
		collapsible?: boolean;
		/** Opens the filter panel, where each of these is changed. */
		onopen: () => void;
		onmaxminutes: (value: number) => void;
		onfavorites: (value: boolean) => void;
		onauthor: (value: string) => void;
		/** The order, when it is not the default one - only where the sort
		 * control is out of sight (the phone, where it lives in the panel). */
		sortLabel?: string;
		/** Back to the default order. */
		onsortreset?: () => void;
		/** The chosen tags, after the panel's chips - the phone's, where no
		 * tag row shows them. */
		tags?: string[];
		ontagremove?: (name: string) => void;
		/** Clears the filters and the tags at once, as the panel's reset
		 * does; offered on the folded bar. */
		onreset?: () => void;
		/** Where the keyboard goes when the last control in the row removes
		 * itself, and the row with it - the "Filters" button on the overview,
		 * not the search field, whose focus would open its suggestions. */
		focusfallback?: () => void;
		/** Classes of the row itself, e.g. desktop's tray. */
		class: string;
	} = $props();

	type Chip = { key: string; label: string; remove: () => void; removeLabel?: string };

	/** Whether everything, spelled out, fits the row on one line. */
	let fits = $state(true);
	let row = $state<HTMLDivElement | null>(null);
	/** Off-screen copy of the spelled-out row, for its width. */
	let twin = $state<HTMLDivElement | null>(null);

	const person = $derived(authors.find((entry) => entry.username === author));
	/** One entry per filter panel section that is set, the order first. */
	const chips = $derived(
		(
			[
				// The order leads: it is not a filter, but on a phone this is the
				// only place that says why the grid stands the way it does.
				sortLabel !== '' && {
					key: 'sort',
					label: sortLabel,
					remove: () => onsortreset?.(),
					removeLabel: m.overview_sort_reset()
				},
				maxMinutes > 0 && {
					key: 'time',
					label: m.overview_filter_time_upto({ duration: formatMinutes(maxMinutes) }),
					remove: () => onmaxminutes(0)
				},
				favoritesOnly && {
					key: 'favorites',
					label: m.overview_filter_favorites_heading(),
					remove: () => onfavorites(false)
				},
				author !== '' && {
					key: 'author',
					// A link to someone who has since left still names the username.
					label: person?.displayName ?? author,
					remove: () => onauthor('')
				}
			] as (Chip | false)[]
		).filter((chip): chip is Chip => chip !== false)
	);
	/** The chips that narrow the grid - all but the order, which the bar
	 * shows with an icon but does not count or list as a filter. */
	const filters = $derived(chips.filter((chip) => chip.key !== 'sort'));
	const crowded = $derived(collapsible && chips.length + tags.length >= 2 && !fits);
	// Unfolded by hand, the row stays unfolded only while it stays crowded:
	// any change of `crowded` resets it, so the next crowding folds again.
	// A writable derived - assigning it overrides the value until then.
	let expanded = $derived.by(() => {
		void crowded;
		return false;
	});
	const folded = $derived(crowded && !expanded);

	/** The row's buttons in order, the twin's left out. */
	function controls(): HTMLElement[] {
		return row
			? [...row.querySelectorAll<HTMLElement>('button')].filter((el) => !el.closest('[inert]'))
			: [];
	}

	// Almost every control here removes itself when used: a ×, a tag, the
	// folded bar as it unfolds or clears, "Less" as it folds. Left alone,
	// the keyboard's focus would fall back to the page, so after the update
	// it moves to whatever took the control's place: "Less" hands it to the
	// folded bar, everything else to the control now at the same position,
	// the last one to `focusfallback`. Whenever the control held the focus,
	// which a screen reader's double tap gives it without `:focus-visible`;
	// focusing a button after a pointer draws no ring. The fallback only for
	// the keyboard, though: it leaves the row, and a tap there would move the
	// page out from under the finger.
	async function keepFocus(event: Event) {
		const button = (event.target as Element).closest('button');
		if (!button || document.activeElement !== button) {
			return;
		}
		const keyboard = button.matches(':focus-visible');
		const index = controls().indexOf(button);
		const folding = button.getAttribute('aria-expanded') === 'true';
		// "All" leaves at most the order's chip; its label, not its ×, which
		// a second Enter would take the order away with.
		const clearing = button.hasAttribute('data-clear');
		// A task, not `tick()`: this listener runs before the control's own
		// handler, which Svelte delegates to the document root, so at this
		// point there is no update pending yet for `tick()` to wait for.
		await new Promise((resolve) => setTimeout(resolve));
		if (button.isConnected) {
			return;
		}
		const next = controls();
		const target = folding
			? next.find((el) => el.getAttribute('aria-expanded') === 'false')
			: clearing
				? next[0]
				: next[Math.min(index, next.length - 1)];
		if (target) {
			target.focus();
		} else if (keyboard) {
			focusfallback?.();
		}
	}

	// Folding is for a row that has run out of room, not for a count: one
	// tag and a time limit stay spelled out on any phone. Whether they fit
	// is measured the way TagFilter measures its row - the spelled-out row
	// lives on in an off-screen twin, because while folded it is not there
	// to be measured.
	function measure() {
		if (row && twin) {
			// Fractional widths: whole pixels let a row 0.5px too wide pass.
			fits = twin.getBoundingClientRect().width <= row.getBoundingClientRect().width;
		}
	}

	$effect(() => {
		// Re-measure when the filters or the tags change, not only on resize.
		void chips;
		void tags;
		if (!collapsible || !row) {
			return;
		}
		const resize = new ResizeObserver(() => measure());
		resize.observe(row);
		if (twin) {
			resize.observe(twin);
		}
		measure();
		return () => resize.disconnect();
	});
</script>

{#snippet icon(key: string, size: string)}
	{#if key === 'sort'}
		<ArrowDownWideNarrow class="{size} shrink-0 text-primary" aria-hidden="true" />
	{:else if key === 'time'}
		<Clock class="{size} shrink-0 text-primary" aria-hidden="true" />
	{:else if key === 'favorites'}
		<Star class="{size} shrink-0 fill-current text-primary" aria-hidden="true" />
	{:else if person}
		<!-- As on a card: the picture, or the initial on the person's color. -->
		<PersonMark {person} size="xs" />
	{:else}
		<!-- A link to someone who has since left: no picture, no color. -->
		<UserRound class="{size} shrink-0 text-text-muted" aria-hidden="true" />
	{/if}
{/snippet}

{#snippet full(chip: Chip)}
	<span
		class="inline-flex h-9 max-w-full min-w-0 shrink-0 items-center rounded-pill border border-border bg-surface text-body-sm font-medium text-text"
	>
		<button
			type="button"
			aria-haspopup="dialog"
			onclick={onopen}
			class="inline-flex h-full min-w-0 items-center gap-1.5 rounded-l-pill pr-1 {chip.key ===
				'author' && person
				? 'pl-2'
				: 'pl-3'}"
		>
			{@render icon(chip.key, 'size-4')}
			<span class="truncate">{chip.label}</span>
		</button>
		<button
			type="button"
			onclick={chip.remove}
			aria-label={chip.removeLabel ?? m.overview_filter_remove({ name: chip.label })}
			class="inline-flex h-full shrink-0 items-center rounded-r-pill pr-3 pl-1"
		>
			<X class="size-3.5 opacity-70" aria-hidden="true" />
		</button>
	</span>
{/snippet}

{#snippet tagChip(name: string)}
	<TagChip
		label={name}
		active
		ariaLabel={m.overview_filter_remove({ name })}
		onclick={() => ontagremove?.(name)}
	>
		{#snippet trailing()}
			<X class="size-3.5 shrink-0 opacity-70" aria-hidden="true" />
		{/snippet}
	</TagChip>
{/snippet}

<!-- Light chips, where a tag's is dark: these come from the filter panel,
     and a tap goes back there to change them, while the × drops just this
     one. Two buttons side by side in one pill, since a button cannot hold
     another. -->
<!-- The row is not a control, it only watches the clicks of the ones
     inside it - in the capture phase, a native listener, so it runs before
     their handlers, which Svelte delegates to the document root. -->
<div bind:this={row} onclickcapture={keepFocus} class={className}>
	{#if folded}
		<!-- Folded, everything goes into one bar across the row: a circle per
		     kind of filter says which are on, the tags' with how many - the
		     only kind that can be on more than once - and a tap unfolds them
		     all. Beside it "All ×" clears them, as the panel's reset does. -->
		<div
			class="flex h-9 w-full items-center rounded-pill border border-border bg-surface text-body-sm font-medium text-text"
		>
			<button
				type="button"
				aria-expanded="false"
				onclick={() => (expanded = true)}
				class="flex h-full min-w-0 flex-1 items-center gap-1 rounded-l-pill pr-2 pl-1.5"
			>
				<span class="flex items-center gap-1" aria-hidden="true">
					{#each chips as chip (chip.key)}
						<span class="grid size-6 place-items-center rounded-full border border-border">
							{@render icon(chip.key, 'size-3')}
						</span>
					{/each}
					{#if tags.length > 0}
						<span
							class="inline-flex h-6 items-center gap-1 rounded-pill border border-border pr-2 pl-1.5 text-caption font-semibold"
						>
							<TagIcon class="size-3 shrink-0 text-primary" />
							{tags.length}
						</span>
					{/if}
				</span>
				<span class="sr-only">
					{m.overview_active_count({ count: filters.length + tags.length })}:
					{[...filters.map((chip) => chip.label), ...tags].join(', ')}
				</span>
				<ChevronDown class="ml-auto size-4 shrink-0 opacity-70" aria-hidden="true" />
			</button>
			{#if onreset}
				<span aria-hidden="true" class="my-2 w-px self-stretch bg-border"></span>
				<button
					type="button"
					data-clear
					onclick={onreset}
					aria-label={m.overview_filter_clear_all_label()}
					class="inline-flex h-full shrink-0 items-center gap-1.5 rounded-r-pill pr-3.5 pl-3 text-text-muted"
				>
					{m.overview_filter_clear_all()}
					<X class="size-3.5" aria-hidden="true" />
				</button>
			{/if}
		</div>
	{:else}
		{#each chips as chip (chip.key)}
			{@render full(chip)}
		{/each}
		{#each tags as name (name)}
			{@render tagChip(name)}
		{/each}
	{/if}
	{#if crowded && expanded}
		<!-- The same toggle the desktop tag row folds with. -->
		<button
			type="button"
			aria-expanded="true"
			onclick={() => (expanded = false)}
			class="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-pill border border-border bg-surface px-3.5 text-body-sm font-medium text-text-muted transition hover:text-text"
		>
			{m.overview_tags_less()}
			<ChevronDown class="size-4 rotate-180" aria-hidden="true" />
		</button>
	{/if}
	{#if collapsible}
		<!-- Off-screen twin, inert and hidden from assistive tech like
		     TagFilter's (see the note there on why both). -->
		<div class="absolute h-0 w-0 overflow-hidden" inert aria-hidden="true">
			<!-- `w-max`: inside the zero-width box the chips' `max-w-full`
			     would otherwise cap every one of them at 0px, and the row
			     would always seem to fit. -->
			<div bind:this={twin} class="flex w-max gap-2">
				{#each chips as chip (chip.key)}
					{@render full(chip)}
				{/each}
				{#each tags as name (name)}
					{@render tagChip(name)}
				{/each}
			</div>
		</div>
	{/if}
</div>
