<script lang="ts" generics="T extends string | number">
	import type { Component } from 'svelte';
	import type { LucideProps } from '@lucide/svelte';
	import { Tooltip } from 'bits-ui';

	type Option = { value: T; label: string; icon: Component<LucideProps> };

	let {
		value,
		options,
		label,
		small = false,
		onchange
	}: {
		value: T;
		options: Option[];
		/** Accessible name of the group. */
		label: string;
		/** The phone's 32px pill in the running head, not the 44px one beside "Filters". */
		small?: boolean;
		onchange: (value: T) => void;
	} = $props();

	let buttons: HTMLButtonElement[] = $state([]);
	const index = $derived(options.findIndex((option) => option.value === value));

	// Roving tabindex, as the WAI-ARIA radio group pattern expects (see
	// SegmentedControl): only the checked option is in the tab order, the
	// arrows move and check.
	function handleKey(event: KeyboardEvent, index: number) {
		const delta =
			event.key === 'ArrowRight' || event.key === 'ArrowDown'
				? 1
				: event.key === 'ArrowLeft' || event.key === 'ArrowUp'
					? -1
					: 0;
		if (delta === 0) {
			return;
		}
		event.preventDefault();
		const next = (index + delta + options.length) % options.length;
		onchange(options[next].value);
		buttons[next]?.focus();
	}
</script>

<!--
	Icons only, so each option is named by its label and the label shows as a
	tooltip under a mouse; the tooltip says the same words, so it is not the
	button's description too, or a screen reader would read them twice. The
	checked option is in the accent's sand, like the current page in the
	bottom nav: the light surface the settings' segmented control uses
	hardly stood out from the track in dark mode. Like the bottom nav's, it
	is one pill under the icons that glides to the chosen one: the options
	are equal squares, so each step is one square and the gap between them
	(instant under reduced motion). The track has the shape
	of "Filters" beside it, and each option the corner concentric with the
	track's: its radius less the border and the padding.
-->
<Tooltip.Provider delayDuration={300}>
	<div
		role="radiogroup"
		aria-label={label}
		class="relative inline-flex shrink-0 items-center gap-0.5 rounded-md border border-border bg-surface {small
			? 'p-px'
			: 'p-[3px]'}"
	>
		{#if index >= 0}
			<span
				aria-hidden="true"
				class="absolute bg-accent transition-transform duration-240 ease-glide motion-reduce:transition-none {small
					? 'top-px left-px size-7 rounded-sm'
					: 'top-[3px] left-[3px] size-9 rounded-[calc(var(--radius-md)-4px)]'}"
				style:transform="translateX(calc({index} * (100% + var(--spacing) * 0.5)))"
			></span>
		{/if}
		{#each options as option, i (option.value)}
			{@const checked = option.value === value}
			<Tooltip.Root>
				<Tooltip.Trigger>
					{#snippet child({ props })}
						<button
							{...props}
							bind:this={buttons[i]}
							aria-describedby={undefined}
							type="button"
							role="radio"
							aria-checked={checked}
							aria-label={option.label}
							tabindex={checked ? 0 : -1}
							onclick={() => onchange(option.value)}
							onkeydown={(event) => handleKey(event, i)}
							class="relative inline-flex items-center justify-center transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {small
								? 'size-7 rounded-sm'
								: 'size-9 rounded-[calc(var(--radius-md)-4px)]'} {checked
								? 'text-accent-foreground'
								: 'hover-tint text-text-muted'}"
						>
							<option.icon class={small ? 'size-4' : 'size-[18px]'} aria-hidden="true" />
						</button>
					{/snippet}
				</Tooltip.Trigger>
				<Tooltip.Portal>
					<Tooltip.Content
						side="bottom"
						sideOffset={6}
						class="z-50 rounded-md bg-inverse px-2.5 py-1.5 text-micro text-inverse-foreground shadow-dialog"
					>
						{option.label}
					</Tooltip.Content>
				</Tooltip.Portal>
			</Tooltip.Root>
		{/each}
	</div>
</Tooltip.Provider>
