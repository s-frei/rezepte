<script lang="ts">
	import type { Component, Snippet } from 'svelte';
	import type { LucideProps } from '@lucide/svelte';

	// Where a channel stands on this account or device: a round icon in the
	// tone of the news, a title, a hint, and what to do about it.
	let {
		icon: Icon,
		tone,
		title,
		hint,
		action,
		note
	}: {
		icon: Component<LucideProps>;
		tone: 'good' | 'neutral' | 'warn';
		title: string;
		hint: string;
		/** A button; full width under the text on phones. */
		action?: Snippet;
		/** A line under the whole box, such as the answer to that button; a live region. */
		note?: string;
	} = $props();

	const toneClass = {
		good: 'bg-success-soft text-success-foreground',
		neutral: 'bg-accent text-accent-foreground',
		warn: 'bg-destructive-soft text-destructive'
	};
</script>

<div class="rounded-lg border border-border bg-surface-elevated px-4 py-3">
	<div class="flex flex-col gap-3 md:flex-row md:items-center md:gap-4">
		<div class="flex min-w-0 flex-1 items-center gap-3">
			<span class="flex size-9 shrink-0 items-center justify-center rounded-full {toneClass[tone]}">
				<Icon class="size-4.5" aria-hidden="true" />
			</span>
			<div class="min-w-0">
				<p class="text-body font-semibold wrap-anywhere">{title}</p>
				<p class="mt-0.5 text-caption text-text-muted">{hint}</p>
			</div>
		</div>
		{#if action}
			<!-- On phones the button is as wide as the box and its label may
			     not fit: "Auf diesem Gerät aktivieren" needs 184px, a 320px
			     screen leaves 200. Below 232px of box it drops its icon and
			     may wrap to a second line rather than spill out of the pill. -->
			<div class="shrink-0 max-md:@container">
				<div
					class="flex *:w-full *:max-md:h-auto *:max-md:min-h-9 *:max-md:px-3 *:max-md:py-1.5 *:max-md:text-center md:*:w-auto [&_svg]:hidden md:[&_svg]:block @min-[14.5rem]:[&_svg]:block"
				>
					{@render action()}
				</div>
			</div>
		{/if}
	</div>
	<!-- Always there, empty without a note: screen readers announce a live
	     region's text changing, rarely one inserted along with its text. -->
	<p class={note ? 'mt-2 text-caption text-text-muted' : ''} role="status">{note ?? ''}</p>
</div>
