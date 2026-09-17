<script lang="ts">
	import Search from 'lucide-svelte/icons/search';
	import X from 'lucide-svelte/icons/x';
	import { m } from '$lib/paraglide/messages';

	let {
		value = $bindable(''),
		onsearch,
		id = 'overview-search'
	}: {
		/** Current field contents (bindable so callers can e.g. reset it). */
		value?: string;
		/** Called 250ms after the field settles, and immediately on clear. */
		onsearch: (value: string) => void;
		/** Id of the text field, so a caller can focus it (`?focus=search`). */
		id?: string;
	} = $props();

	let inputNode: HTMLInputElement | undefined;
	let debounceTimer: ReturnType<typeof setTimeout> | undefined;

	function handleInput() {
		clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => onsearch(value), 250);
	}

	function clear() {
		value = '';
		clearTimeout(debounceTimer);
		onsearch('');
		inputNode?.focus();
	}

	function registerInput(node: HTMLInputElement) {
		inputNode = node;
		return () => {
			clearTimeout(debounceTimer);
			inputNode = undefined;
		};
	}
</script>

<div class="flex h-12 w-full items-center gap-2 rounded-pill bg-surface px-4 md:w-[460px]">
	<Search class="size-5 shrink-0 text-text-muted" aria-hidden="true" />
	<input
		{id}
		{@attach registerInput}
		bind:value
		oninput={handleInput}
		type="text"
		inputmode="search"
		autocomplete="off"
		spellcheck="false"
		maxlength="100"
		placeholder={m.overview_search_placeholder()}
		aria-label={m.overview_search_label()}
		class="min-w-0 flex-1 bg-transparent text-body outline-none placeholder:text-text-muted"
	/>
	{#if value}
		<button type="button" onclick={clear} aria-label={m.common_remove()} class="shrink-0">
			<X class="size-4 text-text-muted" aria-hidden="true" />
		</button>
	{/if}
	<kbd
		aria-hidden="true"
		class="hidden shrink-0 rounded-sm border border-border px-1.5 py-0.5 text-micro text-text-muted md:inline-block"
	>
		⌘K
	</kbd>
</div>
