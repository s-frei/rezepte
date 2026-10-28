<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import Command from '@lucide/svelte/icons/command';
	import Search from '@lucide/svelte/icons/search';
	import TagIcon from '@lucide/svelte/icons/tag';
	import X from '@lucide/svelte/icons/x';
	import type { Tag } from '$lib/api/recipes';
	import { m } from '$lib/paraglide/messages';
	import { palette } from '$lib/palette.svelte';
	import { rankTags } from '$lib/recipe/tag-suggestions';

	let {
		value = $bindable(''),
		onsearch,
		tags = [],
		active = [],
		ontoggle,
		id = 'overview-search',
		class: className = ''
	}: {
		/** Current field contents (bindable so callers can e.g. reset it). */
		value?: string;
		/** Called 250ms after the field settles, and immediately on clear. */
		onsearch: (value: string) => void;
		/** Tags the field suggests while it has focus: the ones that still
		 * narrow the grid under its other filters, plus the chosen ones. */
		tags?: Tag[];
		/** Currently selected tag names; suggested with a check. */
		active?: string[];
		/** Turns a suggested tag on or off. Without it the field suggests nothing. */
		ontoggle?: (name: string) => void;
		/** Id of the text field, for a label or a caller that needs to reach it. */
		id?: string;
		/** Extra classes appended to the wrapper, e.g. to constrain its width. */
		class?: string;
	} = $props();

	let inputNode: HTMLInputElement | undefined;
	let wrapperNode: HTMLDivElement | undefined;
	let debounceTimer: ReturnType<typeof setTimeout> | undefined;

	let open = $state(false);
	/** -1 means "nothing chosen yet", so Enter searches what was typed. */
	let highlighted = $state(-1);

	const query = $derived(value.trim().toLocaleLowerCase());
	/** The text row first while there is text, then the tags that match it. */
	const rows = $derived(
		ontoggle
			? [
					...(query ? [{ kind: 'text' as const }] : []),
					...rankTags(tags, query).map((tag) => ({ kind: 'tag' as const, tag }))
				]
			: []
	);
	const listboxOpen = $derived(open && rows.length > 0);
	// The rows can shrink under a highlight while the list is open - the tags
	// are refetched whenever a filter changes - so every read goes through
	// this, which drops a highlight that no longer has a row.
	const current = $derived(highlighted < rows.length ? highlighted : -1);

	// Below md the page is a phone: the field scrolls to the top on focus so
	// the suggestions get the room between it and the keyboard, and searching
	// the text drops the keyboard so the grid it changed comes into view.
	// Picking a tag keeps it: the next tag is usually what comes next.
	const isPhone = () => matchMedia('(max-width: 767.98px)').matches;

	$effect(() => {
		if (!listboxOpen || current < 0) {
			return;
		}
		document.getElementById(`${id}-option-${current}`)?.scrollIntoView({ block: 'nearest' });
	});

	function handleInput() {
		open = true;
		highlighted = -1;
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
		highlighted = -1;
	}

	function searchText() {
		clearTimeout(debounceTimer);
		onsearch(value);
		close();
		if (isPhone()) {
			inputNode?.blur();
		}
	}

	// The text found the tag, it is not a second filter on top of it: it is
	// cleared, and the bound value has reached the page by the time
	// `ontoggle` fetches, so that fetch already runs without it.
	function toggle(name: string) {
		clearTimeout(debounceTimer);
		value = '';
		ontoggle?.(name);
		close();
	}

	function choose(index: number) {
		const row = rows[index];
		if (!row) {
			return;
		}
		if (row.kind === 'text') {
			searchText();
		} else {
			toggle(row.tag.name);
		}
	}

	function handleFocus() {
		open = true;
		if (ontoggle && isPhone()) {
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
					highlighted = (current + 1) % rows.length;
				}
				break;
			case 'ArrowUp':
				if (rows.length > 0) {
					event.preventDefault();
					open = true;
					highlighted = (current - 1 + rows.length) % rows.length;
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
					open = false;
					highlighted = -1;
				}
				break;
		}
	}

	function registerInput(node: HTMLInputElement) {
		inputNode = node;
		return () => {
			clearTimeout(debounceTimer);
			inputNode = undefined;
		};
	}

	function registerWrapper(node: HTMLDivElement) {
		wrapperNode = node;
		return () => {
			wrapperNode = undefined;
		};
	}

	/** The tag name split around the first match, so the match can be set bold. */
	function split(name: string) {
		const at = query ? name.toLocaleLowerCase().indexOf(query) : -1;
		return at < 0
			? { before: name, match: '', after: '' }
			: {
					before: name.slice(0, at),
					match: name.slice(at, at + query.length),
					after: name.slice(at + query.length)
				};
	}
</script>

<!-- `min-w-0` is what lets this shrink in the row it shares with the filter
     button. A flex item's automatic minimum size is its min-content width,
     and the `<input>` inside contributes its `size="20"` default (166px at
     this font) to that no matter that it carries `min-w-0` itself - so the
     bar bottomed out at 226px and pushed the 88px filter button 46px past
     the right edge at 320px.

     `md:relative` decides how wide the suggestions are: from md they hang
     under this field at its width, below md this wrapper is not positioned,
     so they reach out to the caller's row - which the overview makes
     `relative`, giving the list the width of the recipe cards. -->
<div
	{@attach registerWrapper}
	class="flex h-12 w-full min-w-0 scroll-mt-4 items-center gap-2 rounded-pill bg-surface px-4 md:relative md:w-[460px] {className}"
>
	<Search class="size-5 shrink-0 text-text-muted" aria-hidden="true" />
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
		onblur={() => {
			open = false;
			highlighted = -1;
		}}
		onkeydown={handleKeydown}
		type="text"
		inputmode="search"
		autocomplete="off"
		spellcheck="false"
		maxlength="100"
		placeholder={m.overview_search_placeholder()}
		aria-label={m.overview_search_label()}
		role={ontoggle ? 'combobox' : undefined}
		aria-expanded={ontoggle ? listboxOpen : undefined}
		aria-controls={ontoggle ? `${id}-listbox` : undefined}
		aria-autocomplete={ontoggle ? 'list' : undefined}
		aria-activedescendant={listboxOpen && current >= 0 ? `${id}-option-${current}` : undefined}
		class="min-w-0 flex-1 bg-transparent text-body outline-none placeholder:text-text-muted"
	/>
	{#if value}
		<!-- The negative margins grow the tap target to 32x44px without moving the
		     icon; wider would reach over the end of the text. -->
		<button
			type="button"
			onclick={clear}
			aria-label={m.common_remove()}
			class="-mx-2 -my-3.5 grid h-11 w-8 shrink-0 place-items-center"
		>
			<X class="size-4 text-text-muted" aria-hidden="true" />
		</button>
	{/if}
	<button
		type="button"
		onclick={() => (palette.open = true)}
		aria-label={m.palette_open()}
		class="hidden shrink-0 rounded-sm border border-border px-1.5 py-0.5 text-micro text-text-muted transition hover:text-text md:inline-flex"
	>
		<!-- The icon rather than the "⌘" character: no shipped face carries
		     that glyph, so a fallback font set it larger than the K and with
		     its own wide side bearings. -->
		<kbd class="inline-flex items-center gap-px"
			><Command class="size-2.5" strokeWidth={2.5} aria-hidden="true" />K</kbd
		>
	</button>
	{#if listboxOpen}
		<!-- Read like a cookbook's index: the name, a dotted leader, and where
		     a book prints the page, how many recipes carry the tag. A chosen
		     tag trades its tag icon for a check; picking it again drops it.
		     For assistive tech it is a multi-select listbox: `aria-selected`
		     marks the chosen tags - announced where `aria-checked` on an
		     option is not, by VoiceOver above all - and the arrow keys' place
		     travels by `aria-activedescendant` alone. The text row is an
		     action, not a choice, so it carries no `aria-selected`. -->
		<!-- The frame is rounded and clips; the list scrolls inside its
		     padding, so a visible scrollbar runs inside the straight part of
		     the edge instead of squaring it off. 8px down a 20px radius the
		     curve is 4px in, which the 6px side padding clears. -->
		<div
			class="absolute inset-x-0 top-full z-30 mt-2 overflow-hidden rounded-2xl border border-popover-border bg-popover px-1.5 py-2 shadow-dialog"
		>
			<ul
				id="{id}-listbox"
				role="listbox"
				aria-multiselectable="true"
				aria-label={m.overview_search_suggestions()}
				onpointerdown={(event) => {
					// Keeps focus in the field so its blur doesn't close the list
					// before the option's click lands (see TagInput).
					event.preventDefault();
				}}
				class="max-h-[50dvh] overflow-y-auto overscroll-contain md:max-h-[26rem]"
			>
				{#each rows as row, index (row.kind === 'text' ? 'text' : `tag:${row.tag.name}`)}
					<!-- The keys belong to the field, which drives the options through
				     aria-activedescendant; the click is the pointer's way in. -->
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<li
						id="{id}-option-{index}"
						role="option"
						aria-selected={row.kind === 'tag' ? active.includes(row.tag.name) : undefined}
						onclick={() => choose(index)}
						class="flex min-h-11 cursor-pointer items-start gap-2.5 rounded-lg px-3 py-2.5 text-body {index ===
						current
							? 'bg-background'
							: ''} {row.kind === 'text' && rows.length > 1 ? 'mb-1 border-b border-border' : ''}"
					>
						{#if row.kind === 'text'}
							<Search class="mt-1 size-4 shrink-0 text-text-muted" aria-hidden="true" />
							<span class="min-w-0 [overflow-wrap:anywhere]">
								{m.overview_search_text({ query: value.trim() })}
							</span>
						{:else}
							{@const on = active.includes(row.tag.name)}
							{@const parts = split(row.tag.name)}
							{#if on}
								<Check class="mt-1 size-4 shrink-0 text-primary" aria-hidden="true" />
							{:else}
								<TagIcon class="mt-1 size-4 shrink-0 text-text-muted" aria-hidden="true" />
							{/if}
							<span
								class="line-clamp-2 min-w-0 [overflow-wrap:anywhere] {on ? 'font-semibold' : ''}"
								>{parts.before}<b class="font-bold">{parts.match}</b>{parts.after}</span
							>
							<span
								aria-hidden="true"
								class="mb-1.5 min-w-4 flex-1 self-end border-b-2 border-dotted border-handle"
							></span>
							<span class="self-end text-body-sm text-text-muted">{row.tag.count}</span>
						{/if}
					</li>
				{/each}
			</ul>
		</div>
	{/if}
</div>
