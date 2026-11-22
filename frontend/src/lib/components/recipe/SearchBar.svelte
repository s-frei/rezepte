<script lang="ts">
	import Carrot from '@lucide/svelte/icons/carrot';
	import Check from '@lucide/svelte/icons/check';
	import Command from '@lucide/svelte/icons/command';
	import Search from '@lucide/svelte/icons/search';
	import TagIcon from '@lucide/svelte/icons/tag';
	import X from '@lucide/svelte/icons/x';
	import { MediaQuery } from 'svelte/reactivity';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fly } from 'svelte/transition';
	import { untrack } from 'svelte';
	import { listIngredients, type IngredientSuggestion } from '#lib/api/ingredients.js';
	import type { PantryItem, Tag } from '#lib/api/recipes.js';
	import { m } from '#lib/paraglide/messages.js';
	import { palette } from '#lib/palette.svelte.js';
	import { matchedPart } from '#lib/recipe/step-references.js';
	import { rankTags } from '#lib/recipe/tag-suggestions.js';
	import { notChips, tickCovers } from '#lib/recipe/ticks.js';

	let {
		value = $bindable(''),
		onsearch,
		tags,
		active,
		ontoggle,
		ingredients,
		have,
		onaddingredient,
		onremoveingredient,
		id = 'overview-search',
		class: className = ''
	}: {
		/** Current field contents (bindable so callers can e.g. reset it). */
		value?: string;
		/** Called 250ms after the field settles, and immediately on clear. */
		onsearch: (value: string) => void;
		/** Tags the field suggests while it has focus: the ones that still
		 * narrow the grid under its other filters, plus the chosen ones. */
		tags: Tag[];
		/** Currently selected tag names; suggested with a check. */
		active: string[];
		/** Turns a suggested tag on or off. */
		ontoggle: (name: string) => void;
		/** Ingredient chips, shown inside the field. */
		ingredients: string[];
		/** Ticked ingredients; a suggestion they cover says it is ticked. */
		have: PantryItem[];
		/** Adds a suggested ingredient as a chip, by its name and the key it matches by. */
		onaddingredient: (name: string, key: string) => void;
		/** Drops a chip, by its ✕ or Backspace in the empty field. */
		onremoveingredient: (name: string) => void;
		/** Id of the text field, for a label or a caller that needs to reach it. */
		id?: string;
		/** Extra classes appended to the wrapper, e.g. to constrain its width. */
		class?: string;
	} = $props();

	let inputNode: HTMLInputElement | undefined;
	let wrapperNode = $state<HTMLDivElement>();
	let wrapperHeight = $state(0);
	let debounceTimer: ReturnType<typeof setTimeout> | undefined;

	let open = $state(false);
	/** The highlighted row by its key (`rowKey`), not its place: ingredient
	 * suggestions land late, between the text row and the tags, and a highlight
	 * kept by index would move to another row under the arrow keys' feet.
	 * `null` means "nothing chosen yet", so Enter searches what was typed. */
	let highlighted = $state<string | null>(null);

	const query = $derived(value.trim().toLocaleLowerCase());

	/** The server's last answer, with the text it answers: the rows show it
	 * only while that is still the text, so an earlier word's suggestions
	 * never stand under a later one. */
	let suggested = $state<{ q: string; items: IngredientSuggestion[] }>({ q: '', items: [] });
	/** What each chip also covers, for its tooltip: known from the pick, or
	 * asked for once for a chip that came with the URL. */
	let alsoOf = $state<Record<string, string[]>>({});
	$effect(() => {
		for (const name of ingredients) {
			const lower = name.toLocaleLowerCase();
			if (untrack(() => name in alsoOf)) continue;
			alsoOf[name] = [];
			listIngredients(lower, 4).then(
				(items) => {
					const item = items.find((i) => i.key === lower || i.name.toLocaleLowerCase() === lower);
					if (item && item.also.length > 0) alsoOf[name] = item.also;
				},
				() => {
					// Without it the chip has no tooltip, as before the fetch.
				}
			);
		}
	});

	/** The chip a long press shows the tooltip of, on a touch screen, while
	 * the finger stays and for 3 s at most. */
	let pressed = $state<string | null>(null);
	let pressTimer: ReturnType<typeof setTimeout> | undefined;
	function press(event: PointerEvent, name: string) {
		release();
		if (event.pointerType !== 'touch') return;
		pressTimer = setTimeout(() => {
			pressed = name;
			pressTimer = setTimeout(release, 3000);
		}, 500);
	}
	function release() {
		clearTimeout(pressTimer);
		pressed = null;
	}

	$effect(() => {
		const q = query;
		// Asks for one more per chip, since the chips are left out below.
		const limit = 4 + ingredients.length;
		if (!q) {
			return;
		}
		// The next text, or the field going away, aborts this one; a request
		// it aborted answers with nothing.
		const controller = new AbortController();
		const timer = setTimeout(() => {
			listIngredients(q, limit, { signal: controller.signal }).then(
				(items) => (suggested = { q, items }),
				() => {
					if (!controller.signal.aborted) suggested = { q, items: [] };
				}
			);
		}, 150);
		return () => {
			clearTimeout(timer);
			controller.abort();
		};
	});

	const ingredientSuggestions = $derived(
		notChips(suggested.q === query ? suggested.items : [], ingredients).slice(0, 4)
	);

	/** The text row first while there is text, then ingredients, then tags. */
	const rows = $derived([
		...(query ? [{ kind: 'text' as const }] : []),
		...(query ? ingredientSuggestions : []).map((item) => ({ kind: 'ingredient' as const, item })),
		// Without text the whole list stays: on a phone it is the only way
		// to browse the tags.
		...rankTags(tags, query)
			.slice(0, query ? 3 : undefined)
			.map((tag) => ({ kind: 'tag' as const, tag }))
	]);
	const listboxOpen = $derived(open && rows.length > 0);
	// The rows change under a highlight while the list is open - the tags are
	// refetched whenever a filter changes, the ingredients land late - so
	// every read goes through this: the highlighted row's place now, or -1
	// once it has no row.
	const current = $derived(rows.findIndex((row) => rowKey(row) === highlighted));

	function rowKey(row: (typeof rows)[number]): string {
		return row.kind === 'text'
			? 'text'
			: row.kind === 'ingredient'
				? `ingredient:${row.item.key}`
				: `tag:${row.tag.name}`;
	}

	// Below md the page is a phone: the field scrolls to the top on focus so
	// the suggestions get the room between it and the keyboard, and searching
	// the text drops the keyboard so the grid it changed comes into view.
	// Picking a tag keeps it: the next tag is usually what comes next.
	const phone = new MediaQuery('max-width: 767.98px');

	const placeholder = $derived(
		ingredients.length === 0
			? m.overview_search_placeholder()
			: phone.current
				? m.search_placeholder_short()
				: m.search_placeholder_chips()
	);

	$effect(() => {
		if (!listboxOpen || current < 0) {
			return;
		}
		document.getElementById(`${id}-option-${current}`)?.scrollIntoView({ block: 'nearest' });
	});

	function handleInput() {
		open = true;
		highlighted = null;
		clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => onsearch(value), 250);
	}

	function clear() {
		value = '';
		clearTimeout(debounceTimer);
		onsearch('');
		inputNode?.focus();
	}

	function close() {
		open = false;
		highlighted = null;
	}

	function searchText() {
		clearTimeout(debounceTimer);
		onsearch(value);
		close();
		if (phone.current) {
			inputNode?.blur();
		}
	}

	// The text found the tag, it is not a second filter on top of it: it is
	// cleared, and the bound value has reached the page by the time
	// `ontoggle` fetches, so that fetch already runs without it.
	function toggle(name: string) {
		clearTimeout(debounceTimer);
		value = '';
		ontoggle(name);
		close();
	}

	// The same for an ingredient: the text found it and goes.
	function addIngredient(item: IngredientSuggestion) {
		clearTimeout(debounceTimer);
		value = '';
		alsoOf[item.name] = item.also;
		onaddingredient(item.name, item.key);
		close();
		// A phone drops the keyboard, so the chips' cards come into view.
		if (phone.current) {
			inputNode?.blur();
		}
	}

	function choose(index: number) {
		const row = rows[index];
		if (!row) {
			return;
		}
		if (row.kind === 'text') {
			searchText();
		} else if (row.kind === 'ingredient') {
			addIngredient(row.item);
		} else {
			toggle(row.tag.name);
		}
	}

	/** Set while a removed chip hands the focus back, which opens nothing. */
	let quietFocus = false;

	// A chip's ✕ takes itself away: the field it stood in takes the focus,
	// without opening the list or scrolling the page as a tap on it would.
	function removeChip(name: string) {
		onremoveingredient(name);
		quietFocus = true;
		inputNode?.focus();
		quietFocus = false;
	}

	function handleFocus() {
		if (quietFocus) {
			return;
		}
		open = true;
		if (phone.current) {
			wrapperNode?.scrollIntoView({
				block: 'start',
				behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth'
			});
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		switch (event.key) {
			case 'ArrowDown':
				if (rows.length > 0) {
					event.preventDefault();
					open = true;
					highlighted = rowKey(rows[(current + 1) % rows.length]);
				}
				break;
			case 'ArrowUp':
				if (rows.length > 0) {
					event.preventDefault();
					open = true;
					highlighted = rowKey(rows[(current - 1 + rows.length) % rows.length]);
				}
				break;
			case 'Enter':
				event.preventDefault();
				if (listboxOpen && current >= 0) {
					choose(current);
				} else {
					searchText();
				}
				break;
			case 'Escape':
				if (listboxOpen) {
					event.preventDefault();
					close();
				}
				break;
			case 'Backspace': {
				const last = ingredients.at(-1);
				if (value === '' && last !== undefined) {
					event.preventDefault();
					onremoveingredient(last);
				}
				break;
			}
		}
	}

	function registerInput(node: HTMLInputElement) {
		inputNode = node;
		return () => {
			clearTimeout(debounceTimer);
			inputNode = undefined;
		};
	}
</script>

{#snippet option(
	index: number,
	label: string,
	hint: string,
	count: number,
	selected: boolean | undefined,
	icon: 'carrot' | 'tag' | 'check'
)}
	{@const parts = matchedPart(label, query) ?? { before: label, hit: '', after: '' }}
	<!-- Read like a cookbook's index: the name, a dotted leader, and where
	     a book prints the page, how many recipes carry it. -->
	<!-- The keys belong to the field, which drives the options through
	     aria-activedescendant; the click is the pointer's way in. -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div
		id="{id}-option-{index}"
		role="option"
		aria-selected={selected}
		tabindex="-1"
		onclick={() => choose(index)}
		class="flex min-h-11 cursor-pointer items-start gap-2.5 rounded-lg px-3 py-2.5 text-body hover:bg-popover-highlight {index ===
		current
			? 'bg-popover-highlight'
			: ''}"
	>
		{#if icon === 'check'}
			<Check class="mt-1 size-4 shrink-0 text-primary" aria-hidden="true" />
		{:else if icon === 'carrot'}
			<Carrot class="mt-1 size-4 shrink-0 text-text-muted" aria-hidden="true" />
		{:else}
			<TagIcon class="mt-1 size-4 shrink-0 text-text-muted" aria-hidden="true" />
		{/if}
		<span class="flex min-w-0 flex-col [overflow-wrap:anywhere]">
			<span class="line-clamp-2 {selected ? 'font-semibold' : ''}"
				>{parts.before}<b class="font-bold">{parts.hit}</b>{parts.after}</span
			>
			{#if hint}
				<span class="text-caption text-text-muted">{hint}</span>
			{/if}
		</span>
		<span
			aria-hidden="true"
			class="mb-1.5 min-w-4 flex-1 self-end border-b-2 border-dotted border-handle"
		></span>
		<span class="self-end text-body-sm text-text-muted">{count}</span>
	</div>
{/snippet}

<!-- Capture, so a press on another chip starts after this has let go. -->
<svelte:window onpointerdowncapture={release} />

{#snippet heading(text: string, headingId: string)}
	<div id={headingId} class="px-3 pt-2.5 pb-0.5 text-label font-bold text-text-muted uppercase">
		{text}
	</div>
{/snippet}

<!-- `min-w-0` is what lets this shrink in the row it shares with the filter
     button. A flex item's automatic minimum size is its min-content width,
     and the `<input>` inside contributes its `size="20"` default (166px at
     this font) to that no matter that it carries `min-w-0` itself - so the
     bar bottomed out at 226px and pushed the 88px filter button 46px past
     the right edge at 320px.

     The suggestions hang from this wrapper at its exact width, on every
     layout, and grow with it as chips widen it.

     Chips wrap the field onto more lines; the clear button and the ⌘K hint
     sit absolutely on the first line, in the right padding kept for them.
     Past one line the pill would read as a lozenge, so the corners settle
     on the 3xl radius. -->
<div
	bind:this={wrapperNode}
	bind:clientHeight={wrapperHeight}
	class="relative flex min-h-12 w-full min-w-0 scroll-mt-4 flex-wrap items-center gap-1.5 bg-surface py-1.5 pr-10 pl-4 md:pr-[92px] {wrapperHeight >
	56
		? 'rounded-3xl'
		: 'rounded-pill'} {ingredients.length > 0
		? 'max-md:pl-3 md:max-w-[640px] md:flex-1'
		: 'md:w-[460px]'} {className}"
>
	<!-- A phone's field is narrow: with chips it drops the magnifier, and
	     they drop the carrot, so two short names share a line. -->
	<Search
		class="mr-0.5 size-5 shrink-0 text-text-muted {ingredients.length > 0 ? 'max-md:hidden' : ''}"
		aria-hidden="true"
	/>
	{#each ingredients as name, i (name)}
		{@const also = alsoOf[name] ?? []}
		<!-- "Also covers" shows under the chip on hover, on focus of its ✕,
		     which it describes, and on a long press. The press is the touch
		     screen's way in; a keyboard gets the same through the ✕'s focus. -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<span
			onpointerdown={(event) => press(event, name)}
			onpointerup={release}
			onpointercancel={release}
			oncontextmenu={(event) => pressed === name && event.preventDefault()}
			class="group/chip relative inline-flex h-8 items-center gap-1.5 rounded-pill bg-accent pl-2.5 text-body-sm font-semibold whitespace-nowrap text-accent-foreground select-none"
		>
			<Carrot class="size-3.5 shrink-0 max-md:hidden" aria-hidden="true" />
			{name}
			<!-- The hit area reaches 10px back over the chip's own name and 6px
			     out over the gap: 44x44 without touching the next chip or the
			     field. -->
			<button
				type="button"
				onclick={() => removeChip(name)}
				aria-label={m.search_remove_ingredient({ name })}
				aria-describedby={also.length > 0 ? `${id}-chip-${i}-also` : undefined}
				class="relative grid h-8 w-7 place-items-center opacity-80 after:absolute after:-inset-y-1.5 after:-right-1.5 after:-left-2.5 after:content-['']"
			>
				<X class="size-3.5" aria-hidden="true" />
			</button>
			{#if also.length > 0}
				<span
					id="{id}-chip-{i}-also"
					role="tooltip"
					class="pointer-events-none absolute top-full left-0 z-30 mt-1.5 w-max max-w-64 rounded-md border border-popover-border bg-popover px-2.5 py-1.5 text-caption font-normal whitespace-normal text-text shadow-card group-focus-within/chip:block group-hover/chip:block {pressed ===
					name
						? 'block'
						: 'hidden'}"
				>
					{m.search_ingredient_covers({ names: also.join(', ') })}
				</span>
			{/if}
		</span>
	{/each}
	<input
		{id}
		{@attach registerInput}
		bind:value
		oninput={handleInput}
		onfocus={handleFocus}
		onclick={() => {
			// Picking a tag closes the list but keeps the focus here, so a
			// click on the field would not fire focus again.
			open = true;
		}}
		onblur={close}
		onkeydown={handleKeydown}
		type="text"
		inputmode="search"
		autocomplete="off"
		spellcheck="false"
		maxlength="100"
		{placeholder}
		aria-label={m.overview_search_label()}
		role="combobox"
		aria-expanded={listboxOpen}
		aria-controls="{id}-listbox"
		aria-autocomplete="list"
		aria-activedescendant={listboxOpen && current >= 0 ? `${id}-option-${current}` : undefined}
		class="h-[34px] min-w-16 flex-1 bg-transparent pl-1 text-body outline-none placeholder:text-text-muted md:min-w-[120px]"
	/>
	{#if value}
		<!-- 32x44px, centered on the first line. -->
		<button
			type="button"
			onclick={clear}
			aria-label={m.common_remove()}
			class="absolute top-0.5 right-2 grid h-11 w-8 place-items-center md:right-[52px]"
		>
			<X class="size-4 text-text-muted" aria-hidden="true" />
		</button>
	{/if}
	<button
		type="button"
		onclick={() => (palette.open = true)}
		aria-label={m.palette_open()}
		class="absolute top-3 right-3.5 hidden rounded-sm border border-border px-1.5 py-0.5 text-micro text-text-muted transition hover:text-text md:inline-flex"
	>
		<!-- The icon rather than the "⌘" character: no shipped face carries
		     that glyph, so a fallback font set it larger than the K and with
		     its own wide side bearings. -->
		<kbd class="inline-flex items-center gap-px"
			><Command class="size-2.5" strokeWidth={2.5} aria-hidden="true" />K</kbd
		>
	</button>
	{#if listboxOpen}
		<!-- For assistive tech it is a multi-select listbox: `aria-selected`
		     marks the chosen tags - announced where `aria-checked` on an
		     option is not, by VoiceOver above all - and the arrow keys' place
		     travels by `aria-activedescendant` alone. The text row is an
		     action, not a choice, so it carries no `aria-selected`; an
		     ingredient is never chosen while it is offered (a chip leaves the
		     list), so it carries `false`. -->
		<!-- The frame is rounded and clips; the list scrolls inside its
		     padding, so a visible scrollbar runs inside the straight part of
		     the edge instead of squaring it off. 8px down a 20px radius the
		     curve is 4px in, which the 6px side padding clears. -->
		<!-- It drops into place as it opens, and goes at once: the focus has
		     moved on by then, and a list fading out under the next tap would
		     still catch it. Under reduced motion it only fades. -->
		<div
			in:fly={{ y: prefersReducedMotion.current ? 0 : -4, duration: 140, easing: cubicOut }}
			class="absolute inset-x-0 top-full z-30 mt-2 overflow-hidden rounded-2xl border border-popover-border bg-popover px-1.5 py-2 shadow-dialog"
		>
			<div
				id="{id}-listbox"
				role="listbox"
				tabindex="-1"
				aria-multiselectable="true"
				aria-label={m.overview_search_suggestions()}
				onpointerdown={(event) => {
					// Keeps focus in the field so its blur doesn't close the list
					// before the option's click lands (see TagInput).
					event.preventDefault();
				}}
				class="max-h-[50dvh] overflow-y-auto overscroll-contain md:max-h-[26rem]"
			>
				{#if query}
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<div
						id="{id}-option-0"
						role="option"
						aria-selected={undefined}
						tabindex="-1"
						onclick={() => choose(0)}
						class="flex min-h-11 cursor-pointer items-start gap-2.5 rounded-lg px-3 py-2.5 text-body hover:bg-popover-highlight {current ===
						0
							? 'bg-popover-highlight'
							: ''} {rows.length > 1 ? 'mb-1 border-b border-border' : ''}"
					>
						<Search class="mt-1 size-4 shrink-0 text-text-muted" aria-hidden="true" />
						<span class="flex min-w-0 flex-col [overflow-wrap:anywhere]">
							{m.overview_search_text({ query: value.trim() })}
							<!-- Where a search looks, so a hit in an ingredient is no surprise. -->
							<span class="text-caption text-text-muted">{m.overview_search_text_fields()}</span>
						</span>
					</div>
				{/if}
				{#if rows.some((row) => row.kind === 'ingredient')}
					<div role="group" aria-labelledby="{id}-ingredients">
						{@render heading(m.search_add_ingredient(), `${id}-ingredients`)}
						{#each rows as row, index (index)}
							{#if row.kind === 'ingredient'}
								{@render option(
									index,
									row.item.name,
									tickCovers(have, row.item.key)
										? m.search_ingredient_ticked()
										: row.item.also.length > 0
											? m.search_ingredient_also({ names: row.item.also.join(', ') })
											: '',
									row.item.count,
									false,
									'carrot'
								)}
							{/if}
						{/each}
					</div>
				{/if}
				{#if rows.some((row) => row.kind === 'tag')}
					<div role="group" aria-labelledby="{id}-tags">
						{@render heading(m.search_filter_tag(), `${id}-tags`)}
						{#each rows as row, index (index)}
							{#if row.kind === 'tag'}
								{@const on = active.includes(row.tag.name)}
								{@render option(index, row.tag.name, '', row.tag.count, on, on ? 'check' : 'tag')}
							{/if}
						{/each}
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>
