<script lang="ts">
	import type { LucideProps } from '@lucide/svelte';
	import type { Component, Snippet } from 'svelte';

	let {
		icon: Icon,
		label,
		value,
		control,
		tone = 'default',
		stack = false
	}: {
		icon: Component<LucideProps>;
		label: string;
		/** The value or a short hint, in caption muted under the label. */
		value?: Snippet;
		/** One quiet control at the row's end. */
		control?: Snippet;
		/** `attention` marks a fact worth a second look (a dormant account), never an error. */
		tone?: 'default' | 'danger' | 'attention';
		/** The control takes its own full-width line on phones (the theme row). */
		stack?: boolean;
	} = $props();

	const tones = {
		default: { tile: 'bg-background text-text-muted', label: '' },
		danger: { tile: 'bg-background text-destructive', label: 'text-destructive' },
		attention: { tile: 'bg-accent text-accent-foreground', label: 'text-accent-foreground' }
	};
</script>

<!--
	One shape for every row: icon tile, label over value, one control. On
	phones (420px and below) the tile goes; an `attention` row then carries a
	small accent chip of its icon on the label's line, which tells it apart
	without color and keeps its label in line with the others. Nothing here
	ever truncates: label and value wrap between words (a long address
	anywhere), and once the label column would get narrower than 7rem (a 320px
	phone beside a wide control, or 200% text) the control drops to a line of
	its own, at the row's end like every control, instead of squeezing it. A
	control's label never wraps.
-->
<div class="flex flex-wrap items-center gap-x-3.5 gap-y-3 py-3.5">
	<span
		aria-hidden="true"
		class="flex size-[38px] shrink-0 items-center justify-center rounded-md max-[420px]:hidden {tones[
			tone
		].tile}"
	>
		<Icon class="size-[18px]" aria-hidden="true" />
	</span>
	<span class="grid min-w-0 flex-1 basis-28 grid-cols-[minmax(0,1fr)] gap-px">
		<span class="text-body font-semibold wrap-break-word hyphens-auto {tones[tone].label}">
			{#if tone === 'attention'}
				<span
					aria-hidden="true"
					class="mr-1.5 inline-flex size-[1lh] items-center justify-center rounded-sm bg-accent align-top min-[421px]:hidden"
				>
					<Icon class="size-3.5" aria-hidden="true" />
				</span>
			{/if}{label}
		</span>
		{#if value}
			<span class="text-caption wrap-break-word text-text-muted">{@render value()}</span>
		{/if}
	</span>
	{#if control}
		<!-- Below md the 36px controls grow to the 44px touch floor: the
		     `sm` Button, pill Select and menu trigger are h-9, a `sm`
		     SegmentedControl's segments h-7 inside its 4px track. Below sm
		     a `sm` Button's 16px sides become 14px: at 200% text on a 360px
		     phone "Zurücksetzen" otherwise ran 4px past the row. -->
		<div
			class="ml-auto max-w-full shrink-0 whitespace-nowrap max-md:[&_.h-7]:h-9 max-md:[&_.h-9]:h-11 max-sm:[&_.px-4]:px-3.5 {stack
				? 'max-[420px]:basis-full'
				: ''}"
		>
			{@render control()}
		</div>
	{/if}
</div>
