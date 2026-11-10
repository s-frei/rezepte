<script lang="ts">
	import type { Snippet } from 'svelte';
	import X from '@lucide/svelte/icons/x';
	import { m } from '#lib/paraglide/messages.js';

	let {
		label,
		active = false,
		count,
		onclick,
		removable = false,
		onremove,
		removeLabel,
		leading,
		trailing,
		ariaLabel
	}: {
		label: string;
		active?: boolean;
		count?: number;
		onclick?: (event: MouseEvent) => void;
		removable?: boolean;
		onremove?: () => void;
		/** Names the × by what it removes; "Remove" alone when not given. */
		removeLabel?: string;
		/** Rendered before the label on a filter chip - the author filter
		 * puts a dot in the same color its recipes carry on their cards,
		 * so the two read as one thing. */
		leading?: Snippet;
		/** Rendered after the label on a filter chip, in place of the count -
		 * the phone overview marks a chosen tag with the × that removes it. */
		trailing?: Snippet;
		/** Names a filter chip by what a tap does. Given, the chip is a plain
		 * button rather than a pressed toggle: the phone overview's chosen
		 * tags only ever go away when tapped. */
		ariaLabel?: string;
	} = $props();

	// A chip with a click handler is a filter chip (overview tag list); one
	// without is a plain display chip (recipe tags, editor tag list).
	const interactive = $derived(onclick !== undefined);
</script>

{#if interactive}
	<button
		type="button"
		{onclick}
		aria-pressed={ariaLabel ? undefined : active}
		aria-label={ariaLabel}
		class="inline-flex h-9 max-w-full items-center gap-1.5 rounded-pill px-3.5 text-body-sm font-medium transition {active
			? 'bg-inverse text-inverse-foreground'
			: 'border border-border bg-surface text-text'}"
	>
		{#if leading}
			{@render leading()}
		{/if}
		<!-- A long tag name gives way to an ellipsis rather than pushing the
		     chip past its row. -->
		<span class="min-w-0 truncate">{label}</span>
		{#if count !== undefined}
			<span class={active ? 'text-inverse-muted' : 'text-text-muted'}>{count}</span>
		{/if}
		{#if trailing}
			{@render trailing()}
		{/if}
	</button>
{:else}
	<span
		class="inline-flex max-w-full items-center gap-1.5 rounded-pill bg-accent px-3 py-1 text-caption font-semibold text-accent-foreground"
	>
		<span class="min-w-0 truncate">{label}</span>
		{#if count !== undefined}
			<span class="opacity-70">{count}</span>
		{/if}
		{#if removable}
			<!-- Below md the button's hit area is the ::after, reaching past the
			     chip so the visible chip stays small: 46px wide, and 38px high,
			     which is the pitch of `TagInput`'s wrapped rows - any higher and
			     it would reach over the × of the chip above or below. At 70%
			     the × is 3.4:1 on the chip in light (60% was 2.8:1). -->
			<button
				type="button"
				onclick={onremove}
				aria-label={removeLabel ?? m.common_remove()}
				class="relative opacity-70 transition after:absolute after:-inset-x-4 after:-inset-y-3 hover:opacity-100 md:after:hidden"
			>
				<X class="size-3.5" />
			</button>
		{/if}
	</span>
{/if}
