<script lang="ts">
	import Search from '@lucide/svelte/icons/search';
	import X from '@lucide/svelte/icons/x';
	import { m } from '$lib/paraglide/messages';
	import { palette } from '$lib/palette.svelte';

	let {
		value = $bindable(''),
		onsearch,
		id = 'overview-search',
		class: className = ''
	}: {
		/** Current field contents (bindable so callers can e.g. reset it). */
		value?: string;
		/** Called 250ms after the field settles, and immediately on clear. */
		onsearch: (value: string) => void;
		/** Id of the text field, so a caller can focus it (`?focus=search`). */
		id?: string;
		/** Extra classes appended to the wrapper, e.g. to constrain its width. */
		class?: string;
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

<!-- `min-w-0` is what lets this shrink in the row it shares with the filter
     button. A flex item's automatic minimum size is its min-content width,
     and the `<input>` inside contributes its `size="20"` default (166px at
     this font) to that no matter that it carries `min-w-0` itself - so the
     bar bottomed out at 226px and pushed the 88px filter button 46px past
     the right edge at 320px. -->
<div
	class="flex h-12 w-full min-w-0 items-center gap-2 rounded-pill bg-surface px-4 md:w-[460px] {className}"
>
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
	<button
		type="button"
		onclick={() => (palette.open = true)}
		aria-label={m.palette_open()}
		class="hidden shrink-0 rounded-sm border border-border px-1.5 py-0.5 text-micro text-text-muted transition hover:text-text md:inline-block"
	>
		<kbd>⌘K</kbd>
	</button>
</div>
