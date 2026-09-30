<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ResolvedPathname } from '$app/types';
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import Checkbox from '$lib/components/ui/Checkbox.svelte';

	let {
		checked = $bindable(false),
		disabled = false,
		title,
		tags,
		thumb,
		href,
		hrefLabel,
		note = null,
		trailing
	}: {
		checked?: boolean;
		disabled?: boolean;
		title: string;
		tags: string[];
		thumb: string | null;
		/** Opens a recipe in a new tab; null leaves the row without the button. */
		href: ResolvedPathname | null;
		hrefLabel: string;
		/** Shown in place of the tags when set: a message that must be read whole. */
		note?: string | null;
		/** What the dotted leader points at: a photo count, a state, a pill. */
		trailing: Snippet;
	} = $props();

	let brokenThumb = $state<string | null>(null);
</script>

<!--
	One entry of a register, the way a cookbook index reads: title, a dotted
	leader, the number at the end. The label spans everything but the link,
	so a click anywhere on the row ticks it.
-->
<li class="flex items-center gap-3 py-2 [&+&]:border-t [&+&]:border-dashed [&+&]:border-border">
	<label class="flex min-w-0 flex-1 cursor-pointer items-center gap-3">
		<Checkbox bind:checked {disabled} label={title} />
		<!-- A cover a file carries may not decode; the plain tile stands in
		     rather than the browser's broken-image icon. -->
		{#if thumb && brokenThumb !== thumb}
			<img
				src={thumb}
				alt=""
				class="size-9 shrink-0 rounded-sm object-cover md:size-11"
				loading="lazy"
				onerror={() => (brokenThumb = thumb)}
			/>
		{:else}
			<span class="size-9 shrink-0 rounded-sm bg-accent md:size-11" aria-hidden="true"></span>
		{/if}
		<!-- From md up, two lines at any title length: title, leader, count,
		     then the tags - a grid rather than a wrapping flex row, so a long
		     title shrinks and ends in an ellipsis instead of pushing the count
		     onto a line of its own. On a phone the title takes up to two
		     lines, and the tags and the count share the next one until the
		     count - a pill, a state - needs the room; then it wraps below
		     rather than squeezing the tags to a letter. A note - why an import
		     failed - replaces the tags, since it must be read whole. -->
		<span
			class="min-w-0 flex-1 md:grid md:grid-cols-[minmax(0,max-content)_minmax(1.25rem,1fr)_auto] md:items-baseline md:gap-x-2 md:[grid-template-areas:'title_leader_trail'_'tags_tags_tags']"
		>
			<span
				class="line-clamp-2 font-display text-body break-words md:block md:truncate md:[grid-area:title]"
				{title}>{title}</span
			>
			<span
				class="hidden -translate-y-1 border-b-[1.5px] border-dotted border-border md:block md:[grid-area:leader]"
				aria-hidden="true"
			></span>
			<span class="flex flex-wrap items-baseline gap-x-2 md:contents">
				{#if note}
					<span
						class="line-clamp-2 min-w-16 flex-1 text-micro break-words text-destructive md:[grid-area:tags]"
						>{note}</span
					>
				{:else}
					<span class="min-w-16 flex-1 truncate text-micro text-text-muted md:[grid-area:tags]"
						>{tags.join(' · ')}</span
					>
				{/if}
				<span
					class="ml-auto max-w-full truncate text-right tabular-nums md:ml-0 md:max-w-56 md:[grid-area:trail]"
					>{@render trailing()}</span
				>
			</span>
		</span>
	</label>
	{#if href}
		<a
			{href}
			target="_blank"
			rel="noopener"
			aria-label={hrefLabel}
			class="flex size-7 shrink-0 items-center justify-center rounded-pill border border-border bg-surface-elevated text-text-muted transition hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		>
			<ArrowUpRight class="size-4" aria-hidden="true" />
		</a>
	{/if}
</li>
