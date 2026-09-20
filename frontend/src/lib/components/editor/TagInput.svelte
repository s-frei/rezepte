<script lang="ts">
	import { onMount } from 'svelte';
	import { Label } from 'bits-ui';
	import { listTags } from '$lib/api/recipes';
	import TagChip from '$lib/components/ui/TagChip.svelte';
	import { m } from '$lib/paraglide/messages';
	import { MAX_TAGS, normaliseTag } from '$lib/recipe/form';
	import { tagSuggestions } from '$lib/recipe/tag-suggestions';

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
	let open = $state(false);
	/** -1 means "nothing chosen yet", so Enter commits what was typed. */
	let highlighted = $state(-1);

	const atLimit = $derived(tags.length >= MAX_TAGS);
	const suggestions = $derived(tagSuggestions({ query, known, selected: tags }));

	/** The rows as rendered: existing tags first, the creation row last. */
	const rows = $derived([
		...suggestions.matches.map((name) => ({ name, isNew: false })),
		...(suggestions.create ? [{ name: suggestions.create, isNew: true }] : [])
	]);
	const listboxOpen = $derived(open && rows.length > 0);

	// The list scrolls once it outgrows `max-h-64`, so arrowing past the
	// visible rows has to bring the chosen one along.
	$effect(() => {
		if (!listboxOpen || highlighted < 0) {
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
			})
			.catch(() => {});
	});

	function add(candidate: string) {
		const tag = normaliseTag(candidate);
		query = '';
		open = false;
		highlighted = -1;
		if (tag === '' || tags.includes(tag) || atLimit) {
			return;
		}
		tags = [...tags, tag];
	}

	function remove(tag: string) {
		tags = tags.filter((entry) => entry !== tag);
	}

	function commitTyped() {
		// Clicking "Speichern" blurs the field; discarding the text here is
		// what used to lose the tag the user had just typed.
		if (query.trim() !== '') {
			add(query);
		}
		open = false;
		highlighted = -1;
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
			case ',':
				event.preventDefault();
				// Only an arrow-key choice beats what was typed - otherwise a new
				// tag that is a prefix of an existing one could never be created.
				add(highlighted >= 0 && listboxOpen ? rows[highlighted].name : query);
				break;
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

<div class="space-y-1.5">
	<Label.Root for={id} class="text-caption font-semibold">{m.editor_section_tags()}</Label.Root>
	<div class="relative">
		<div
			class="flex min-h-11 flex-wrap items-center gap-1.5 rounded-md border bg-surface-elevated px-2 py-1.5 {error
				? 'border-[1.5px] border-destructive'
				: 'border-border'}"
		>
			{#each tags as tag (tag)}
				<TagChip label={tag} removable onremove={() => remove(tag)} />
			{/each}
			{#if atLimit}
				<span class="px-2 text-caption text-text-muted">
					{m.editor_tag_limit_reached({ count: MAX_TAGS })}
				</span>
			{:else}
				<input
					{id}
					bind:value={query}
					oninput={() => {
						open = true;
						highlighted = -1;
					}}
					onkeydown={handleKeydown}
					onfocus={() => (open = true)}
					onblur={commitTyped}
					type="text"
					autocomplete="off"
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
					class="min-w-32 flex-1 bg-transparent px-2 py-1 text-body-sm outline-none placeholder:text-text-muted"
				/>
			{/if}
		</div>
		{#if listboxOpen}
			<ul
				id="{id}-listbox"
				role="listbox"
				aria-label={m.editor_tag_suggestions()}
				onpointerdown={(event) => {
					// Keeps focus in the field so `onblur` doesn't commit the
					// half-typed text before the option's handler runs. On the
					// list itself, so its padding and scrollbar are covered too.
					event.preventDefault();
				}}
				class="absolute inset-x-0 top-full z-30 mt-1 max-h-64 overflow-y-auto rounded-md bg-surface-elevated py-1 shadow-dialog"
			>
				{#each rows as row, index (row.isNew ? `new:${row.name}` : row.name)}
					<li
						id="{id}-option-{index}"
						role="option"
						aria-selected={index === highlighted}
						onpointerdown={(event) => {
							// Keeps focus in the field so `onblur` doesn't fire first;
							// pointer events fire before mousedown and the focus
							// shift on every modern engine, touch included.
							event.preventDefault();
							add(row.name);
						}}
						class="cursor-pointer px-3 py-2 text-body-sm {index === highlighted
							? 'bg-background font-semibold'
							: ''}"
					>
						{#if row.isNew}
							<span class="text-text-muted">{m.editor_tag_create({ name: row.name })}</span>
						{:else}
							{row.name}
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</div>
	{#if error}
		<p id="{id}-error" class="text-micro font-medium text-destructive">{error}</p>
	{:else if !atLimit}
		<p id="{id}-hint" class="text-micro text-text-muted">{m.editor_tag_hint()}</p>
	{/if}
</div>
