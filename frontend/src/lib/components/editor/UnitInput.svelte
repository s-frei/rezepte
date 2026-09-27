<script lang="ts">
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import { m } from '$lib/paraglide/messages';
	import { unitOptions, unitSuggestions } from '$lib/recipe/units';

	// Free text with a list of suggestions, drawn by the app rather than the
	// browser: a native datalist cannot be styled, shows nothing over a value
	// that already matches, does not reopen on a second tap and on iOS lives
	// in the keyboard bar. The listbox follows TagInput's pattern.
	let {
		value = $bindable(''),
		id,
		class: fieldClass = '',
		onkeydown
	}: {
		value?: string;
		/** Id of the text field; also the prefix for the listbox and its options. */
		id: string;
		/** The row's field classes, so the unit looks like its neighbors. */
		class?: string;
		/** The row's own keys (Backspace in an empty row), for whatever the list does not take. */
		onkeydown?: (event: KeyboardEvent) => void;
	} = $props();

	let input = $state<HTMLInputElement>();
	let open = $state(false);
	/** False right after opening, so the list starts complete; typing narrows it. */
	let typed = $state(false);
	/** -1 means "nothing chosen yet", so Enter keeps what was typed. */
	let highlighted = $state(-1);

	const units = unitSuggestions();
	const options = $derived(unitOptions(units, value, typed));
	const listboxOpen = $derived(open && options.length > 0);

	$effect(() => {
		if (!listboxOpen || highlighted < 0) {
			return;
		}
		document.getElementById(`${id}-option-${highlighted}`)?.scrollIntoView({ block: 'nearest' });
	});

	/** Every way in opens the whole list, with the current unit marked. */
	function openList() {
		open = true;
		typed = false;
		const current = value.trim().toLowerCase();
		highlighted = units.findIndex((unit) => unit.toLowerCase() === current);
	}

	function close() {
		open = false;
		highlighted = -1;
	}

	function pick(unit: string) {
		value = unit;
		close();
	}

	function handleKeydown(event: KeyboardEvent) {
		switch (event.key) {
			case 'ArrowDown':
				event.preventDefault();
				if (!listboxOpen) {
					openList();
				} else {
					highlighted = (highlighted + 1) % options.length;
				}
				return;
			case 'ArrowUp':
				event.preventDefault();
				if (!listboxOpen) {
					openList();
				} else {
					highlighted = (highlighted - 1 + options.length) % options.length;
				}
				return;
			case 'Enter':
				if (listboxOpen && highlighted >= 0) {
					event.preventDefault();
					pick(options[highlighted]);
					return;
				}
				close();
				break;
			case 'Escape':
				if (listboxOpen) {
					// Only dismisses the list; swallowed so it does not also reach
					// the editor's discard guard.
					event.preventDefault();
					event.stopPropagation();
					close();
					return;
				}
				break;
			case 'Tab':
				close();
				break;
		}
		onkeydown?.(event);
	}
</script>

<div class="relative min-w-0">
	<input
		{id}
		bind:this={input}
		bind:value
		oninput={() => {
			open = true;
			typed = true;
			highlighted = -1;
		}}
		onfocus={openList}
		onclick={() => {
			// A second tap on a field that kept its focus fires no focus event.
			if (!listboxOpen) {
				openList();
			}
		}}
		onblur={close}
		onkeydown={handleKeydown}
		type="text"
		autocomplete="off"
		autocapitalize="off"
		spellcheck="false"
		role="combobox"
		aria-expanded={listboxOpen}
		aria-controls="{id}-listbox"
		aria-autocomplete="list"
		aria-activedescendant={listboxOpen && highlighted >= 0
			? `${id}-option-${highlighted}`
			: undefined}
		aria-label={m.editor_ingredient_unit()}
		placeholder={m.editor_ingredient_unit()}
		class="{fieldClass} pr-9"
	/>
	<button
		type="button"
		tabindex="-1"
		aria-label={m.editor_unit_show_suggestions()}
		onpointerdown={(event) => {
			// Keeps the focus in the field, so opening does not blur it first.
			event.preventDefault();
			if (document.activeElement !== input) {
				input?.focus();
			} else if (listboxOpen) {
				close();
			} else {
				openList();
			}
		}}
		class="absolute inset-y-0 right-0 flex w-9 items-center justify-center text-text-muted"
	>
		<ChevronDown
			class="size-4 transition-transform {listboxOpen ? 'rotate-180' : ''}"
			aria-hidden="true"
		/>
	</button>
	{#if listboxOpen}
		<ul
			id="{id}-listbox"
			role="listbox"
			aria-label={m.editor_unit_suggestions()}
			onpointerdown={(event) => {
				// Keeps the focus in the field, so the list does not close
				// under the finger. The pick waits for the click, which a swipe
				// through the list never produces.
				event.preventDefault();
			}}
			class="absolute inset-x-0 top-full z-30 mt-1 max-h-64 overflow-y-auto rounded-md bg-surface-elevated py-1 shadow-dialog"
		>
			{#each options as unit, index (unit)}
				<!-- The keys belong to the field, which drives the options through
				     aria-activedescendant; the click is the pointer's way in. -->
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<li
					id="{id}-option-{index}"
					role="option"
					aria-selected={index === highlighted}
					onclick={() => pick(unit)}
					class="cursor-pointer px-3 py-2 text-body-sm {index === highlighted
						? 'bg-background font-semibold'
						: ''}"
				>
					{unit}
				</li>
			{/each}
		</ul>
	{/if}
</div>
