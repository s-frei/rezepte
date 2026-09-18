<script lang="ts">
	import Check from 'lucide-svelte/icons/check';
	import ChevronDown from 'lucide-svelte/icons/chevron-down';
	import { Select as BitsSelect } from 'bits-ui';

	type Option = { value: string; label: string };

	let {
		value = $bindable(''),
		options,
		label,
		disabled = false,
		onchange,
		class: className = ''
	}: {
		value?: string;
		options: Option[];
		/** Accessible name of the trigger. */
		label: string;
		disabled?: boolean;
		/** Fires when the user picks an option (not on programmatic changes). */
		onchange?: (value: string) => void;
		/** Extra trigger classes, e.g. the role pill colours. */
		class?: string;
	} = $props();

	const selectedLabel = $derived(options.find((o) => o.value === value)?.label ?? '');
</script>

<BitsSelect.Root type="single" bind:value {disabled} onValueChange={(next) => onchange?.(next)}>
	<BitsSelect.Trigger
		aria-label={label}
		class="inline-flex h-8 items-center gap-1.5 rounded-pill px-3 text-caption font-semibold transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-50 {className}"
	>
		{selectedLabel}
		<ChevronDown class="size-3.5" aria-hidden="true" />
	</BitsSelect.Trigger>
	<BitsSelect.Portal>
		<BitsSelect.Content
			preventScroll={false}
			sideOffset={6}
			class="z-50 min-w-[160px] rounded-2xl bg-surface p-1.5 shadow-dialog"
		>
			{#each options as option (option.value)}
				<BitsSelect.Item
					value={option.value}
					label={option.label}
					class="flex h-10 cursor-default items-center justify-between rounded-sm px-3 text-body-sm text-text outline-none data-highlighted:bg-background"
				>
					{#snippet children({ selected })}
						{option.label}
						{#if selected}
							<Check class="size-4 text-primary" aria-hidden="true" />
						{/if}
					{/snippet}
				</BitsSelect.Item>
			{/each}
		</BitsSelect.Content>
	</BitsSelect.Portal>
</BitsSelect.Root>
