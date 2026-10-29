<script lang="ts" generics="T extends string">
	import type { Component } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import type { LucideProps } from '@lucide/svelte';

	type IconComponent = Component<LucideProps>;

	type Option = { value: T; label: string; icon?: IconComponent };

	let {
		value = $bindable(),
		options,
		label,
		disabled = false,
		onchange
	}: {
		value: T;
		options: Option[];
		/** Accessible name of the group. */
		label: string;
		/**
		 * `aria-disabled`, not the native attribute: the checked segment stays
		 * in the tab order and every click and arrow key is dropped.
		 */
		disabled?: boolean;
		/** Fires after the user picks a different segment. */
		onchange?: (value: T) => void;
	} = $props();

	let buttons: HTMLButtonElement[] = $state([]);
	let group = $state<HTMLDivElement>();
	let pillEl = $state<HTMLSpanElement>();
	// The checked segment's place in the group. The segments differ in width
	// (their labels, and the icons from 280px of container width), so the
	// pill is measured rather than moved by a fraction like the bottom nav's.
	let pill = $state<{ x: number; w: number } | null>(null);
	const glide = $derived(prefersReducedMotion.current ? 0 : 300);

	function measure() {
		const button = buttons[options.findIndex((option) => option.value === value)];
		pill = button ? { x: button.offsetLeft, w: button.offsetWidth } : null;
	}

	// Measured on mount, on an outside change of `value` and on every resize,
	// all without motion: only a pick glides (`select`).
	$effect(() => {
		void value;
		void options;
		measure();
	});

	$effect(() => {
		if (!group) return;
		const observer = new ResizeObserver(measure);
		observer.observe(group);
		for (const button of buttons) observer.observe(button);
		return () => observer.disconnect();
	});

	function select(next: T) {
		if (disabled || next === value) {
			return;
		}
		const from = pill;
		value = next;
		onchange?.(next);
		measure();
		if (from && pill && pillEl && glide) {
			const easing = getComputedStyle(pillEl).getPropertyValue('--ease-glide').trim() || 'ease';
			pillEl.animate(
				[
					{ translate: `${from.x}px`, width: `${from.w}px` },
					{ translate: `${pill.x}px`, width: `${pill.w}px` }
				],
				{ duration: glide, easing }
			);
		}
	}

	// Roving tabindex, as the WAI-ARIA radio group pattern expects: only the
	// checked segment is in the tab order, arrows move and check.
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
		if (disabled) {
			return;
		}
		const next = (index + delta + options.length) % options.length;
		select(options[next].value);
		buttons[next]?.focus();
	}
</script>

<!--
	Phones get the full width with equal segments: sized by its content the
	control was a constant 297px and overflowed its card on anything narrower
	than 390px (and on wider screens once the reader scales up their system
	font). Desktop keeps the content width, and the control does not shrink
	beside text in a row.

	The icons hang off a container query rather than a screen breakpoint,
	because what decides whether they fit is the width of whatever box the
	control sits in, not the size of the device: three labeled segments plus
	icons need 275px, so they appear from 280px of container width - a 375px
	phone shows them, a 360px one does not. Two segments fit with their icons
	in any box. The container is phones only: from `md` the control sits
	content-sized in rows, where an inline-size container would measure 0.
-->
<div class="shrink-0 max-md:@container">
	<!-- The checked look is one pill behind the labels that glides from the
	     segment it left; on each segment, the background blinked across. -->
	<div
		bind:this={group}
		role="radiogroup"
		aria-label={label}
		aria-disabled={disabled || undefined}
		class="relative flex w-full rounded-pill bg-background p-1 md:inline-flex md:w-auto"
	>
		<span
			bind:this={pillEl}
			aria-hidden="true"
			class="absolute inset-y-1 left-0 rounded-pill bg-surface shadow-card {pill ? '' : 'hidden'}"
			style:translate="{pill?.x ?? 0}px"
			style:width="{pill?.w ?? 0}px"
		></span>
		{#each options as option, i (option.value)}
			{@const checked = option.value === value}
			{@const Icon = option.icon}
			<button
				bind:this={buttons[i]}
				type="button"
				role="radio"
				aria-checked={checked}
				aria-disabled={disabled || undefined}
				tabindex={checked ? 0 : -1}
				onclick={() => select(option.value)}
				onkeydown={(event) => handleKey(event, i)}
				class="relative inline-flex h-9 min-w-0 flex-1 items-center justify-center gap-2 rounded-pill px-2 text-body-sm font-semibold transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:flex-none md:px-4 {checked
					? 'text-text'
					: disabled
						? 'text-text-muted'
						: 'text-text-muted hover:text-text'}"
			>
				{#if Icon}
					<Icon
						class="size-4 shrink-0 {options.length > 2 ? 'hidden md:block @min-[280px]:block' : ''}"
						aria-hidden="true"
					/>
				{/if}
				<span class="truncate">{option.label}</span>
			</button>
		{/each}
	</div>
</div>
