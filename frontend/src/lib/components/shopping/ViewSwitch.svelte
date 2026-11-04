<script lang="ts">
	import BookOpen from '@lucide/svelte/icons/book-open';
	import Carrot from '@lucide/svelte/icons/carrot';
	import { setListView, listView, type ListView } from '#lib/shopping/view.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	let {
		class: className = '',
		compact = false
	}: {
		class?: string;
		/**
		 * The list card's head: from a tablet on a smaller switch beside the
		 * counts. Without it (shopping mode) the switch spans its column everywhere.
		 */
		compact?: boolean;
	} = $props();

	const options = [
		{ value: 'department', label: m.list_view_department(), icon: Carrot },
		{ value: 'recipe', label: m.list_view_recipe(), icon: BookOpen }
	] as const;

	let buttons: HTMLButtonElement[] = $state([]);

	// Roving tabindex like CardSizeControl: the arrows move and check.
	function handleKey(event: KeyboardEvent, index: number) {
		if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return;
		event.preventDefault();
		const next = (index + 1) % options.length;
		setListView(options[next].value as ListView);
		buttons[next]?.focus();
	}
</script>

<!-- CardSizeControl's track with words beside the icons; the checked one in the accent's sand. -->
<div
	role="radiogroup"
	aria-label={m.list_view_label()}
	class="flex shrink-0 items-center gap-0.5 rounded-md border border-border bg-surface p-[3px] {compact
		? 'md:inline-flex'
		: ''} {className}"
>
	{#each options as option, i (option.value)}
		{@const checked = listView.current === option.value}
		<button
			bind:this={buttons[i]}
			type="button"
			role="radio"
			aria-checked={checked}
			tabindex={checked ? 0 : -1}
			onclick={() => setListView(option.value)}
			onkeydown={(event) => handleKey(event, i)}
			class="inline-flex h-10 flex-1 items-center justify-center gap-2 rounded-[calc(var(--radius-md)-4px)] px-3 text-body-sm font-semibold whitespace-nowrap transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {compact
				? 'md:h-8 md:flex-none md:px-2.5'
				: 'md:h-9'} {checked ? 'bg-accent text-accent-foreground' : 'hover-tint text-text-muted'}"
		>
			<option.icon class="size-4" aria-hidden="true" />
			{option.label}
		</button>
	{/each}
</div>
