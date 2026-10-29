<script lang="ts">
	import type { Component } from 'svelte';
	import type { LucideProps } from '@lucide/svelte';
	import { ToggleGroup } from 'bits-ui';
	import { m } from '$lib/paraglide/messages';

	// "Off | On" for one setting in a list of them, where a row of switches
	// would not say which channel it is about: each side carries the
	// channel's icon. Off sits left and On right, like a switch's track.
	// Disabled is `aria-disabled`, not the native attribute: the toggle stays
	// in the tab order and says why it does not move, and every change is dropped.
	let {
		checked,
		label,
		onIcon,
		offIcon,
		disabled = false,
		onchange
	}: {
		checked: boolean;
		/** Accessible name of the group; the visible text belongs to the caller's row. */
		label: string;
		onIcon: Component<LucideProps>;
		offIcon: Component<LucideProps>;
		disabled?: boolean;
		onchange: (checked: boolean) => void;
	} = $props();

	const items = $derived([
		{ value: 'off', label: m.settings_notifications_off(), Icon: offIcon },
		{ value: 'on', label: m.settings_notifications_on(), Icon: onIcon }
	]);
</script>

<!--
	The value is the caller's `checked`, read back on every render: clicking
	the chosen side asks to deselect it (an empty value), which is dropped,
	so the group never ends up with neither side chosen.
	Full width with equal halves on phones, content width from `md`, as
	`SegmentedControl` does.
-->
<ToggleGroup.Root
	type="single"
	bind:value={
		() => (checked ? 'on' : 'off'),
		(v) => {
			if (!disabled && (v === 'on' || v === 'off')) onchange(v === 'on');
		}
	}
	aria-label={label}
	aria-disabled={disabled || undefined}
	class="flex w-full shrink-0 rounded-pill bg-background p-1 md:inline-flex md:w-auto"
>
	{#each items as { value, label: text, Icon } (value)}
		<ToggleGroup.Item
			{value}
			aria-disabled={disabled || undefined}
			class="{disabled
				? 'pointer-events-none'
				: ''} inline-flex h-9 min-w-0 flex-1 items-center justify-center gap-2 rounded-pill px-2 text-body-sm font-semibold whitespace-nowrap text-text-muted transition hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary data-[state=on]:bg-surface data-[state=on]:text-text data-[state=on]:shadow-card md:flex-none md:px-4"
		>
			<Icon class="size-4 shrink-0" aria-hidden="true" />
			{text}
		</ToggleGroup.Item>
	{/each}
</ToggleGroup.Root>
