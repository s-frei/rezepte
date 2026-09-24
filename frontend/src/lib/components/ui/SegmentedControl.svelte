<script lang="ts" generics="T extends string">
	import type { ComponentType, SvelteComponent } from 'svelte';
	import type { IconProps } from 'lucide-svelte';

	// lucide-svelte 1.0.1 still ships Svelte 4 class components, so the icon
	// slot is typed with `ComponentType`, not Svelte 5's `Component`.
	type IconComponent = ComponentType<SvelteComponent<IconProps>>;

	type Option = { value: T; label: string; icon?: IconComponent };

	let {
		value = $bindable(),
		options,
		label,
		onchange
	}: {
		value: T;
		options: Option[];
		/** Accessible name of the group. */
		label: string;
		/** Fires after the user picks a different segment. */
		onchange?: (value: T) => void;
	} = $props();

	let buttons: HTMLButtonElement[] = $state([]);

	function select(next: T) {
		if (next === value) {
			return;
		}
		value = next;
		onchange?.(next);
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
		const next = (index + delta + options.length) % options.length;
		select(options[next].value);
		buttons[next]?.focus();
	}
</script>

<!--
	Phones get the full width with equal segments: sized by its content the
	control was a constant 297px and overflowed its card on anything narrower
	than 390px (and on wider screens once the reader scales up their system
	font). Desktop keeps the content width.

	The icons hang off a container query rather than a screen breakpoint,
	because what decides whether they fit is the width of whatever box the
	control sits in, not the size of the device: three labeled segments plus
	icons need 275px, so they appear from 280px of container width - a 375px
	phone shows them, a 360px one does not.
-->
<div class="@container">
	<div
		role="radiogroup"
		aria-label={label}
		class="flex w-full rounded-pill bg-background p-1 md:inline-flex md:w-auto"
	>
		{#each options as option, i (option.value)}
			{@const checked = option.value === value}
			{@const Icon = option.icon}
			<button
				bind:this={buttons[i]}
				type="button"
				role="radio"
				aria-checked={checked}
				tabindex={checked ? 0 : -1}
				onclick={() => select(option.value)}
				onkeydown={(event) => handleKey(event, i)}
				class="inline-flex h-9 min-w-0 flex-1 items-center justify-center gap-2 rounded-pill px-2 text-body-sm font-semibold transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:flex-none md:px-4 {checked
					? 'bg-surface text-text shadow-card'
					: 'text-text-muted hover:text-text'}"
			>
				{#if Icon}
					<Icon class="hidden size-4 shrink-0 @min-[280px]:block" aria-hidden="true" />
				{/if}
				<span class="truncate">{option.label}</span>
			</button>
		{/each}
	</div>
</div>
