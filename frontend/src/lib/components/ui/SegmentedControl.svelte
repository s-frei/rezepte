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
		size = 'md',
		track = 'background',
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
		/** `sm` is 36px tall, the one height of a `SettingsRow` control. */
		size?: 'md' | 'sm';
		/**
		 * The ground the track reads on: `background` is a `bg-background`
		 * track for a `surface` panel; `surface` heads a `bg-background`
		 * sheet: a bordered `bg-surface` track with the checked segment in
		 * `primary` - the choice a whole sheet hangs on, so it has to read at
		 * a glance in the dark too. Phones keep the 36px segments (the sheet's
		 * fit steps count on every pixel there; the 44px hit area comes from
		 * `::after` as for every track), `md` grows them to 44px in body type.
		 * It fills its box from `md` too, the segments sharing it equally.
		 */
		track?: 'background' | 'surface';
		/** Fires after the user picks a different segment. */
		onchange?: (value: T) => void;
	} = $props();

	let buttons: HTMLButtonElement[] = $state([]);
	let group = $state<HTMLDivElement>();
	let pillEl = $state<HTMLSpanElement>();
	// The checked segment's place in the group. The segments differ in width
	// (their labels, and the icons from 280px of container width), so the
	// pill is measured rather than moved by a fraction like the bottom nav's.
	let pill = $state<{ x: number; y: number; w: number; h: number } | null>(null);
	const glide = $derived(prefersReducedMotion.current ? 0 : 300);
	const onSurface = $derived(track === 'surface');

	function measure() {
		const button = buttons[options.findIndex((option) => option.value === value)];
		pill = button
			? { x: button.offsetLeft, y: button.offsetTop, w: button.offsetWidth, h: button.offsetHeight }
			: null;
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
					{ translate: `${from.x}px ${from.y}px`, width: `${from.w}px`, height: `${from.h}px` },
					{ translate: `${pill.x}px ${pill.y}px`, width: `${pill.w}px`, height: `${pill.h}px` }
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
	beside text in a row. A label never truncates: when the segments no
	longer fit side by side (200% text on a phone) they wrap onto a second
	line, and the pill follows them there. The track's 24px corners read as
	a pill at one line's height and stay a soft box at two.

	The icons hang off a container query rather than a screen breakpoint,
	because what decides whether they fit is the width of whatever box the
	control sits in, not the size of the device: three labeled segments plus
	icons need 275px, so they appear from 280px of container width - a 375px
	phone shows them, a 360px one does not. Two segments fit with their icons
	in any box. The container is phones only: from `md` the control sits
	content-sized in rows, where an inline-size container would measure 0.
-->
<div class="shrink-0 max-md:@container">
	<!-- On phones each segment's `::after` reaches over the track's 4px
	     above and below it, so a 36px segment hits on 44px. -->
	<!-- The checked look is one pill behind the labels that glides from the
	     segment it left; on each segment, the background blinked across. -->
	<div
		bind:this={group}
		role="radiogroup"
		aria-label={label}
		aria-disabled={disabled || undefined}
		class="relative flex w-full flex-wrap rounded-3xl p-1 {onSurface
			? 'border border-handle bg-surface md:border-[1.5px]'
			: 'bg-background md:inline-flex md:w-auto'}"
	>
		<span
			bind:this={pillEl}
			aria-hidden="true"
			class="absolute top-0 left-0 rounded-pill {onSurface
				? 'bg-primary shadow-cta'
				: 'bg-surface shadow-card'} {pill ? '' : 'hidden'}"
			style:translate="{pill?.x ?? 0}px {pill?.y ?? 0}px"
			style:width="{pill?.w ?? 0}px"
			style:height="{pill?.h ?? 0}px"
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
				class="relative inline-flex {onSurface
					? 'h-9 text-body-sm md:h-11 md:text-body'
					: size === 'sm'
						? 'h-7 text-body-sm'
						: 'h-9 text-body-sm'} min-w-fit flex-1 items-center justify-center gap-2 rounded-pill px-2 font-semibold transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary max-md:after:absolute max-md:after:inset-x-0 max-md:after:-inset-y-1 {onSurface
					? 'duration-300 ease-glide motion-reduce:duration-0'
					: 'md:flex-none md:px-4'} {checked
					? onSurface
						? 'text-primary-foreground'
						: 'text-text'
					: disabled
						? 'text-text-muted'
						: 'text-text-muted hover:text-text'}"
			>
				{#if Icon}
					<Icon
						class="{onSurface ? 'size-4 md:size-4.5' : 'size-4'} shrink-0 {options.length > 2
							? 'hidden md:block @min-[280px]:block'
							: ''}"
						aria-hidden="true"
					/>
				{/if}
				<span>{option.label}</span>
			</button>
		{/each}
	</div>
</div>
