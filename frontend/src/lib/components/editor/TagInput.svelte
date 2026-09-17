<script lang="ts">
	import { onMount } from 'svelte';
	import { Label } from 'bits-ui';
	import { listTags } from '$lib/api/recipes';
	import TagChip from '$lib/components/ui/TagChip.svelte';
	import { m } from '$lib/paraglide/messages';
	import { MAX_TAGS, normaliseTag } from '$lib/recipe/form';

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
	let highlighted = $state(0);

	const atLimit = $derived(tags.length >= MAX_TAGS);

	const suggestions = $derived.by(() => {
		const prefix = normaliseTag(query);
		if (prefix === '') {
			return [];
		}
		return known.filter((name) => name.startsWith(prefix) && !tags.includes(name)).slice(0, 8);
	});

	const listboxOpen = $derived(open && suggestions.length > 0);

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
		highlighted = 0;
		if (tag === '' || tags.includes(tag) || atLimit) {
			return;
		}
		tags = [...tags, tag];
	}

	function remove(tag: string) {
		tags = tags.filter((entry) => entry !== tag);
	}

	function handleInput() {
		open = true;
		highlighted = 0;
	}

	function handleKeydown(event: KeyboardEvent) {
		switch (event.key) {
			case 'ArrowDown':
				if (suggestions.length > 0) {
					event.preventDefault();
					open = true;
					highlighted = (highlighted + 1) % suggestions.length;
				}
				break;
			case 'ArrowUp':
				if (suggestions.length > 0) {
					event.preventDefault();
					open = true;
					highlighted = (highlighted - 1 + suggestions.length) % suggestions.length;
				}
				break;
			case 'Enter':
			case ',':
				// Enter must not submit the surrounding form while the user is
				// still assembling tags.
				event.preventDefault();
				add(listboxOpen ? suggestions[highlighted] : query);
				break;
			case 'Escape':
				if (listboxOpen) {
					// Swallowed so the first Escape only closes the suggestions.
					event.preventDefault();
					event.stopPropagation();
					open = false;
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
					oninput={handleInput}
					onkeydown={handleKeydown}
					onfocus={() => (open = true)}
					onblur={() => (open = false)}
					type="text"
					autocomplete="off"
					spellcheck="false"
					role="combobox"
					aria-expanded={listboxOpen}
					aria-controls="{id}-listbox"
					aria-autocomplete="list"
					aria-activedescendant={listboxOpen ? `${id}-option-${highlighted}` : undefined}
					aria-invalid={error ? 'true' : undefined}
					aria-describedby={error ? `${id}-error` : undefined}
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
				class="absolute inset-x-0 top-full z-30 mt-1 overflow-hidden rounded-md bg-surface-elevated py-1 shadow-dialog"
			>
				{#each suggestions as name, index (name)}
					<li
						id="{id}-option-{index}"
						role="option"
						aria-selected={index === highlighted}
						onmousedown={(event) => {
							// Keeps focus in the field so `onblur` doesn't close the
							// listbox before the option is picked.
							event.preventDefault();
							add(name);
						}}
						class="cursor-pointer px-3 py-2 text-body-sm {index === highlighted
							? 'bg-background font-semibold'
							: ''}"
					>
						{name}
					</li>
				{/each}
			</ul>
		{/if}
	</div>
	{#if error}
		<p id="{id}-error" class="text-micro font-medium text-destructive">{error}</p>
	{/if}
</div>
