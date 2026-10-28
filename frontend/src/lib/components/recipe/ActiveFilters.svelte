<script lang="ts">
	import type { Snippet } from 'svelte';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import Clock from '@lucide/svelte/icons/clock';
	import Star from '@lucide/svelte/icons/star';
	import X from '@lucide/svelte/icons/x';
	import type { Author } from '$lib/api/recipes';
	import { formatMinutes } from '$lib/recipe/format';
	import { userColorClasses } from '$lib/user/color';
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
		children,
		focusfallback,
		class: className = 'flex flex-wrap gap-2'
	}: {
		/** The filter panel's state, as FilterPanel gets it. */
		maxMinutes: number;
		favoritesOnly: boolean;
		author: string;
		/** For the chosen author's display name and color. */
		authors: Author[];
		/** Phones: once the row would need a second line, fold two or more
		 * of these chips into one that unfolds them again. */
		collapsible?: boolean;
		/** Opens the filter panel, where each of these is changed. */
		onopen: () => void;
		onmaxminutes: (value: number) => void;
		onfavorites: (value: boolean) => void;
		onauthor: (value: string) => void;
		/** Rendered after the filter chips, before "Less" - the phone's tags. */
		children?: Snippet;
		/** Where the keyboard goes when the last control in the row removes
		 * itself, and the row with it - the "Filters" button on the overview,
		 * not the search field, whose focus would open its suggestions. */
		focusfallback?: () => void;
		/** Classes of the row itself; `contents` hands the chips to the
		 * caller's own row, as on desktop. */
		class?: string;
	} = $props();

	type Chip = { key: string; label: string; remove: () => void };

	const GAP_PX = 8; // gap-2

	/** Whether every chip, spelled out, fits the row on one line. */
	let fits = $state(true);
	let row = $state<HTMLDivElement | null>(null);
	/** Off-screen copy of the spelled-out filter chips, for their widths. */
	let twin = $state<HTMLDivElement | null>(null);

	const person = $derived(authors.find((entry) => entry.username === author));
	/** One entry per filter panel section that narrows the grid. */
	const chips = $derived(
		[
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
		].filter((chip): chip is Chip => chip !== false)
	);
	const crowded = $derived(collapsible && chips.length >= 2 && !fits);
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
	// folded chip as it unfolds, "Less" as it folds. Left alone, the
	// keyboard's focus would fall back to the page, so after the update it
	// moves to whatever took the control's place: "Less" hands it to the
	// folded chip, everything else to the control now at the same position,
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
			: next[Math.min(index, next.length - 1)];
		if (target) {
			target.focus();
		} else if (keyboard) {
			focusfallback?.();
		}
	}

	// A listener rather than an `onclick` on the row: the row is not a
	// control, it only watches the clicks of the ones inside it, the tags
	// the caller puts there included.
	function registerRow(node: HTMLDivElement) {
		row = node;
		node.addEventListener('click', keepFocus);
		return () => {
			node.removeEventListener('click', keepFocus);
			row = null;
		};
	}

	function registerTwin(node: HTMLDivElement) {
		twin = node;
		return () => {
			twin = null;
		};
	}

	// Folding is for a row that has run out of room, not for a count: one
	// tag and a time limit stay spelled out on any phone. Whether they fit
	// is measured the way TagFilter measures its row - the spelled-out chips
	// live on in an off-screen twin, because while folded they are not in
	// the row to be measured, and everything else in the row (the tags,
	// marked by not being `data-own`) is measured where it stands.
	function measure() {
		if (!row || !twin || twin.children.length !== chips.length) {
			return;
		}
		const others = ([...row.children] as HTMLElement[]).filter(
			(el) => !el.hasAttribute('data-own')
		);
		const widths = [...(twin.children as HTMLCollectionOf<HTMLElement>), ...others].map(
			(el) => el.offsetWidth
		);
		const total = widths.reduce((sum, width) => sum + width, 0) + GAP_PX * (widths.length - 1);
		fits = total <= row.clientWidth;
	}

	$effect(() => {
		// Re-measure when the filters change, not only on resize.
		void chips;
		if (!collapsible || !row) {
			return;
		}
		const resize = new ResizeObserver(() => measure());
		resize.observe(row);
		if (twin) {
			resize.observe(twin);
		}
		// A tag chip coming or going changes what has to fit as well.
		const mutation = new MutationObserver(() => measure());
		mutation.observe(row, { childList: true });
		measure();
		return () => {
			resize.disconnect();
			mutation.disconnect();
		};
	});
</script>

{#snippet icon(key: string)}
	{#if key === 'time'}
		<Clock class="size-4 shrink-0 text-primary" aria-hidden="true" />
	{:else if key === 'favorites'}
		<Star class="size-4 shrink-0 fill-current text-primary" aria-hidden="true" />
	{:else}
		<span
			aria-hidden="true"
			class="size-3 shrink-0 rounded-full ring-1 ring-text-muted/60 {userColorClasses(
				person?.color
			)}"
		></span>
	{/if}
{/snippet}

{#snippet full(chip: Chip, own: boolean)}
	<span
		data-own={own ? '' : undefined}
		class="inline-flex h-9 max-w-full min-w-0 shrink-0 items-center rounded-pill border border-border bg-surface text-body-sm font-medium text-text"
	>
		<button
			type="button"
			aria-haspopup="dialog"
			onclick={onopen}
			class="inline-flex h-full min-w-0 items-center gap-1.5 rounded-l-pill pr-1 pl-3"
		>
			{@render icon(chip.key)}
			<span class="truncate">{chip.label}</span>
		</button>
		<button
			type="button"
			onclick={chip.remove}
			aria-label={m.overview_filter_remove({ name: chip.label })}
			class="inline-flex h-full shrink-0 items-center rounded-r-pill pr-3 pl-1"
		>
			<X class="size-3.5 opacity-70" aria-hidden="true" />
		</button>
	</span>
{/snippet}

<!-- Light chips, where a tag's is dark: these come from the filter panel,
     and a tap goes back there to change them, while the × drops just this
     one. Two buttons side by side in one pill, since a button cannot hold
     another. -->
<div {@attach registerRow} class={className}>
	{#if folded}
		<!-- Folded, the chips' icons overlap in one pill, like avatars, and say
		     which kinds of filter are on; the values are one tap away. -->
		<button
			type="button"
			data-own
			aria-expanded="false"
			onclick={() => (expanded = true)}
			class="inline-flex h-9 shrink-0 items-center gap-2 rounded-pill border border-border bg-surface pr-3.5 pl-1.5 text-body-sm font-medium text-text"
		>
			<span class="flex items-center" aria-hidden="true">
				{#each chips as chip (chip.key)}
					<span
						class="-ml-1.5 grid size-6 place-items-center rounded-full border border-border bg-surface first:ml-0 [&>span]:size-2.5 [&>svg]:size-3"
					>
						{@render icon(chip.key)}
					</span>
				{/each}
			</span>
			{m.overview_filters_count({ count: chips.length })}
			<span class="sr-only">: {chips.map((chip) => chip.label).join(', ')}</span>
		</button>
	{:else}
		{#each chips as chip (chip.key)}
			{@render full(chip, true)}
		{/each}
	{/if}
	{@render children?.()}
	{#if crowded && expanded}
		<!-- The same toggle the desktop tag row folds with. -->
		<button
			type="button"
			data-own
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
		<div data-own class="absolute h-0 w-0 overflow-hidden" inert aria-hidden="true">
			<!-- `w-max`: inside the zero-width box the chips' `max-w-full`
			     would otherwise cap every one of them at 0px, and the row
			     would always seem to fit. -->
			<div {@attach registerTwin} class="flex w-max gap-2">
				{#each chips as chip (chip.key)}
					{@render full(chip, false)}
				{/each}
			</div>
		</div>
	{/if}
</div>
