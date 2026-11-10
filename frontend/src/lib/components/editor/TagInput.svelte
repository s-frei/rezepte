<script lang="ts">
	import { onMount } from 'svelte';
	import { Label } from 'bits-ui';
	import Plus from '@lucide/svelte/icons/plus';
	import TagIcon from '@lucide/svelte/icons/tag';
	import FieldError from '#lib/components/ui/FieldError.svelte';
	import { backOut, cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MediaQuery } from 'svelte/reactivity';
	import { fly, scale } from 'svelte/transition';
	import { listTags } from '#lib/api/recipes.js';
	import TagChip from '#lib/components/ui/TagChip.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { MAX_TAGS, normaliseTag } from '#lib/recipe/form.js';
	import { tagSuggestions, tagToCommit } from '#lib/recipe/tag-suggestions.js';

	let {
		tags = $bindable([]),
		error = null,
		id = 'editor-tags'
	}: {
		tags?: string[];
		error?: string | null;
		/** Id of the text field; also the prefix for the listbox and its options. */
		id?: string;
	} = $props();

	let query = $state('');
	let known = $state<string[]>([]);
	/** How many recipes carry each known tag, shown at the end of its row. */
	let counts = $state(new Map<string, number>());
	let open = $state(false);
	/** -1 means "no row highlighted", so Enter and a comma commit nothing. */
	let highlighted = $state(-1);
	/** Read out by the live region when a tag goes on or comes off. */
	let announcement = $state('');

	// Below md the page is a phone: on focus the label scrolls up to just
	// under the 60px editor bar (the bar's `scroll-padding-top`, and
	// `scroll-mt-2` puts its box 8px below), so the suggestions get the room
	// above the keyboard and the whole field stays in sight.
	const phone = new MediaQuery('max-width: 767.98px');
	let wrapper = $state<HTMLDivElement>();

	const atLimit = $derived(tags.length >= MAX_TAGS);
	const CHIP_DURATION = $derived(prefersReducedMotion.current ? 0 : 200);
	const suggestions = $derived(tagSuggestions({ query, known, selected: tags }));

	/** The existing tags first, creating one last (see `tagSuggestions`). */
	const rows = $derived(suggestions.options);
	const listboxOpen = $derived(open && rows.length > 0);

	// Set by a pointer's highlight, which must not scroll the list: the row
	// would slide away under a still pointer. Not `$state`: nothing renders
	// from it.
	let pointed = false;
	/** Where the pointer last moved, so a scroll under a still pointer
	 * (which some browsers answer with a pointermove) moves no highlight. */
	let lastPointer = '';

	// The list scrolls once it outgrows its height, so arrowing past the
	// visible rows has to bring the chosen one along.
	$effect(() => {
		if (!listboxOpen || highlighted < 0 || pointed) {
			pointed = false;
			return;
		}
		document.getElementById(`${id}-option-${highlighted}`)?.scrollIntoView({ block: 'nearest' });
	});

	// Suggestions are a convenience - if the tag list can't be fetched the
	// field still takes free text, so the error is swallowed rather than
	// turned into a toast the user can do nothing about.
	onMount(() => {
		void listTags()
			.then((items) => {
				known = items.map((tag) => normaliseTag(tag.name));
				counts = new Map(items.map((tag) => [normaliseTag(tag.name), tag.count]));
			})
			.catch(() => {});
	});

	// After a pick the focus stays in the field and the list stays open on
	// the tags still left, the first one highlighted, so the next can be
	// browsed and taken without typing. At the limit the field gives way to
	// the limit note instead.
	function add(candidate: string) {
		const tag = normaliseTag(candidate);
		query = '';
		if (tag !== '' && !tags.includes(tag) && !atLimit) {
			tags = [...tags, tag];
			announcement = m.editor_tag_added({ tag });
		}
		open = !atLimit;
		highlighted = open && rows.length > 0 ? 0 : -1;
	}

	// The pointer moves the same highlight the keys do, as in any listbox;
	// leaving the list keeps it where it was, as a menu does. Only a real
	// move counts (see `lastPointer`).
	function point(event: PointerEvent, index: number) {
		const at = `${event.clientX},${event.clientY}`;
		if (event.pointerType !== 'mouse' || at === lastPointer || index === highlighted) {
			lastPointer = at;
			return;
		}
		lastPointer = at;
		pointed = true;
		highlighted = index;
	}

	function remove(tag: string) {
		tags = tags.filter((entry) => entry !== tag);
		announcement = m.editor_tag_removed({ tag });
	}

	// Typing highlights what Enter takes - the first existing tag that
	// matches, creating only when none does - so the list shows what the
	// key does. Creating is the last row, reached by arrow or tap.
	function handleInput() {
		open = true;
		highlighted = suggestions.preferred;
	}

	function handleFocus() {
		open = true;
		highlighted = suggestions.preferred;
		if (phone.current) {
			wrapper?.scrollIntoView({
				block: 'start',
				behavior: prefersReducedMotion.current ? 'auto' : 'smooth'
			});
		}
	}

	// On a computer the field can sit low in the window, the list then
	// running past its bottom: the page scrolls just far enough to show it.
	// Not on a phone, where the focus scroll has already made the room and
	// an instant scroll here would cut its smooth one short.
	function revealList(node: HTMLElement) {
		if (!phone.current) {
			node.scrollIntoView({ block: 'nearest' });
		}
	}

	// On a phone the list ends 12px above the keyboard: the visual viewport
	// is what the keyboard leaves, and the list follows it as the keyboard
	// opens and the focus scroll moves the field. Never under two rows
	// (88px), never over `max-h-48`.
	function fitAboveKeyboard(node: HTMLElement) {
		const viewport = window.visualViewport;
		if (!phone.current || !viewport) {
			return;
		}
		// Read so the list is fitted again when a chip goes on: a new row of
		// chips moves the list down without a scroll or a resize.
		void tags.length;
		const fit = () => {
			// 21px: the frame's 8px bottom padding and 1px border, and the gap.
			const room = viewport.offsetTop + viewport.height - node.getBoundingClientRect().top - 21;
			node.style.maxHeight = `min(12rem, ${Math.max(88, room)}px)`;
		};
		fit();
		viewport.addEventListener('resize', fit);
		viewport.addEventListener('scroll', fit);
		window.addEventListener('scroll', fit);
		return () => {
			viewport.removeEventListener('resize', fit);
			viewport.removeEventListener('scroll', fit);
			window.removeEventListener('scroll', fit);
		};
	}

	/** The tag name split around the typed text, so that part can be set bold. */
	function split(name: string) {
		const typed = normaliseTag(query);
		const at = typed ? name.indexOf(typed) : -1;
		return at < 0
			? { before: name, match: '', after: '' }
			: {
					before: name.slice(0, at),
					match: name.slice(at, at + typed.length),
					after: name.slice(at + typed.length)
				};
	}

	// Leaving the field - Tab, a tap elsewhere, closing the keyboard, or
	// Save - commits nothing (`tagToCommit`): a tag goes on only when it is
	// chosen. The typed text stays in the field, uncommitted, so nothing is
	// lost and nothing is created behind the user's back; coming back opens
	// the list on it again. Save reads only `tags`, never this text.
	function leave() {
		open = false;
		highlighted = -1;
	}

	// A click on the field while it already has focus opens or closes the
	// list, as in the WAI-ARIA editable combobox pattern: the way to close it
	// on a phone without leaving the field. The click that focuses the field
	// only opens it (`handleFocus`). Typing opens it again.
	let focusedBeforePress = false;

	function toggle() {
		if (!focusedBeforePress) {
			return;
		}
		open = !open;
		highlighted = open ? suggestions.preferred : -1;
	}

	function handleKeydown(event: KeyboardEvent) {
		switch (event.key) {
			case 'ArrowDown':
				if (rows.length > 0) {
					event.preventDefault();
					open = true;
					highlighted = (highlighted + 1) % rows.length;
				}
				break;
			case 'ArrowUp':
				if (rows.length > 0) {
					event.preventDefault();
					open = true;
					highlighted = (highlighted - 1 + rows.length) % rows.length;
				}
				break;
			case 'Enter':
			case ',': {
				event.preventDefault();
				// A comma does what Enter does: it takes the highlighted row, so
				// the two keys never disagree about which tag goes on. With no
				// row highlighted it takes nothing - creating is a choice too.
				const tag = tagToCommit(event.key === ',' ? 'comma' : 'enter', rows, highlighted);
				if (tag !== null) {
					add(tag);
				}
				break;
			}
			case 'Escape':
				if (listboxOpen) {
					// The first Escape only dismisses the suggestions and clears
					// the typed text; swallowed so it doesn't also reach the
					// editor's discard guard.
					event.preventDefault();
					event.stopPropagation();
					query = '';
					open = false;
					highlighted = -1;
				}
				break;
			case 'Backspace':
				if (query === '' && tags.length > 0) {
					event.preventDefault();
					remove(tags[tags.length - 1]);
				}
				break;
		}
	}
</script>

<div bind:this={wrapper} class="scroll-mt-2 space-y-1.5" data-error-key="tags">
	<Label.Root for={id} class="text-caption font-semibold">{m.editor_section_tags()}</Label.Root>
	<div class="relative">
		<!-- Focused, the field looks like `ui/Input` focused: the border turns
		     primary (destructive with an error) and the 2px ring sits 2px
		     outside. Keyed to the text field, not to `focus-within`, so a
		     chip's × keeps its own focus ring and leaves the field quiet. -->
		<div
			class="flex min-h-11 flex-wrap items-center gap-x-1.5 gap-y-3 rounded-md border bg-surface-elevated px-2 py-1.5 transition has-[input:focus-visible]:outline-2 has-[input:focus-visible]:outline-offset-2 md:gap-y-1.5 {error
				? 'border-[1.5px] border-destructive has-[input:focus]:border-destructive has-[input:focus-visible]:outline-destructive'
				: 'border-border has-[input:focus]:border-primary has-[input:focus-visible]:outline-primary'}"
		>
			<!-- A chip springs in when it is added. No flip when one goes: the
			     field after the chips reflows at once, and chips sliding after
			     it ran over its placeholder. The tags the recipe came with are
			     simply there: a local intro does not play on the first render.
			     Wrapped rows stand 12px apart on a phone, room for each ×'s hit
			     area without reaching the row above or below (see `TagChip`). -->
			{#each tags as tag (tag)}
				<span
					in:scale={{ start: 0.9, duration: CHIP_DURATION, easing: backOut }}
					class="inline-flex"
				>
					<TagChip
						label={tag}
						removable
						removeLabel={m.editor_tag_remove({ tag })}
						onremove={() => remove(tag)}
					/>
				</span>
			{/each}
			{#if atLimit}
				<span class="px-2 text-caption text-text-muted">
					{m.editor_tag_limit_reached({ count: MAX_TAGS })}
				</span>
			{:else}
				<input
					{id}
					bind:value={query}
					oninput={handleInput}
					onkeydown={handleKeydown}
					onfocus={handleFocus}
					onblur={leave}
					onpointerdown={(event) =>
						(focusedBeforePress = document.activeElement === event.currentTarget)}
					onclick={toggle}
					type="text"
					autocomplete="off"
					autocapitalize="none"
					enterkeyhint="done"
					spellcheck="false"
					role="combobox"
					aria-expanded={listboxOpen}
					aria-controls="{id}-listbox"
					aria-autocomplete="list"
					aria-activedescendant={listboxOpen && highlighted >= 0
						? `${id}-option-${highlighted}`
						: undefined}
					aria-invalid={error ? 'true' : undefined}
					aria-describedby={error ? `${id}-error` : !atLimit ? `${id}-hint` : undefined}
					placeholder={m.editor_tag_add_placeholder()}
					class="min-h-11 min-w-32 flex-1 bg-transparent px-2 py-1 text-input text-text outline-none placeholder:text-text-muted md:min-h-0"
				/>
			{/if}
		</div>
		{#if listboxOpen}
			<!-- Read like the overview's search (SearchBar): an existing tag is a
			     tag icon, its name with the typed part bold, a dotted leader and
			     how many recipes carry it. Creating one is an action, not a pick,
			     and looks it: the last row, under a rule (a ::before above the
			     row, so the highlight keeps its round corners), a filled
			     `primary` plus, the verb in `accent-foreground` (`primary` text
			     fell below 4.5:1 on the highlight) and the new tag drawn as the
			     chip it will become. The highlight is `background` in light and a
			     7% tint of the text in dark, where `background` lies below the
			     popover and read as a hole rather than a lift. Icons and text line
			     up with the field's text: 1px border, 6px frame and 10px row
			     padding match the field's 1px border and 2 x 8px padding.
			     The frame is rounded and clips; the list scrolls inside its
			     padding, so its scrollbar runs on the straight part of the edge.
			     It drops into place as it opens and goes at once, as SearchBar's
			     does; under reduced motion it only fades. -->
			<div
				{@attach revealList}
				in:fly={{ y: prefersReducedMotion.current ? 0 : -4, duration: 140, easing: cubicOut }}
				class="absolute inset-x-0 top-full z-30 mt-1 overflow-hidden rounded-2xl border border-popover-border bg-popover px-1.5 py-2 shadow-dialog"
			>
				<!-- Out of the tab order: Chromium makes a scrolling box a Tab stop
				     of its own, so Tab left the field for the list, the field's
				     blur closed it, and the focus fell to <body>. -->
				<ul
					id="{id}-listbox"
					role="listbox"
					tabindex="-1"
					aria-label={m.editor_tag_suggestions()}
					onpointerdown={(event) => {
						// Keeps focus in the field so `onblur` doesn't close the
						// list before the option's handler runs. On the list
						// itself, so its scrollbar is covered too.
						event.preventDefault();
					}}
					{@attach fitAboveKeyboard}
					class="max-h-48 overflow-y-auto overscroll-contain md:max-h-64"
				>
					{#each rows as row, index (row.isNew ? `new:${row.name}` : row.name)}
						<!-- The keys belong to the field, which drives the options through
						     aria-activedescendant; the click is the pointer's way in. It
						     waits for the click because a swipe through the list never
						     produces one. -->
						<!-- svelte-ignore a11y_click_events_have_key_events -->
						<li
							id="{id}-option-{index}"
							role="option"
							aria-selected={index === highlighted}
							aria-label={row.isNew ? m.editor_tag_create_label({ name: row.name }) : undefined}
							onclick={() => add(row.name)}
							onpointermove={(event) => point(event, index)}
							class="flex min-h-11 cursor-pointer gap-2.5 rounded-lg px-2.5 text-body {index ===
							highlighted
								? 'bg-background dark:bg-text/7'
								: ''} {row.isNew ? 'items-center py-2' : 'items-start py-2.5'} {row.isNew &&
							rows.length > 1
								? 'relative mt-2 before:absolute before:inset-x-2.5 before:-top-1 before:h-px before:bg-popover-border'
								: ''}"
						>
							{#if row.isNew}
								<span
									aria-hidden="true"
									class="grid size-5 shrink-0 place-items-center rounded-pill bg-primary text-primary-foreground"
								>
									<Plus class="size-3.5" strokeWidth={3} />
								</span>
								<span class="shrink-0 font-semibold text-accent-foreground"
									>{m.editor_tag_create()}</span
								>
								<span class="flex min-w-0"><TagChip label={row.name} /></span>
							{:else}
								{@const parts = split(row.name)}
								<span class="mt-0.5 grid size-5 shrink-0 place-items-center">
									<TagIcon class="size-4 text-text-muted" aria-hidden="true" />
								</span>
								<span class="line-clamp-2 min-w-0 [overflow-wrap:anywhere]"
									>{parts.before}<b class="font-bold">{parts.match}</b>{parts.after}</span
								>
								{#if counts.has(row.name)}
									<span
										aria-hidden="true"
										class="mb-1.5 min-w-4 flex-1 self-end border-b-2 border-dotted border-handle"
									></span>
									<span class="self-end text-body-sm text-text-muted">{counts.get(row.name)}</span>
								{/if}
							{/if}
						</li>
					{/each}
				</ul>
			</div>
		{/if}
	</div>
	<p class="sr-only" aria-live="polite">{announcement}</p>
	{#if error}
		<FieldError id="{id}-error" message={error} />
	{:else if !atLimit}
		<p id="{id}-hint" class="text-micro text-text-muted">{m.editor_tag_hint()}</p>
	{/if}
</div>
