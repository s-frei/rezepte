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

	// The default trigger typography steps aside when the caller passes its
	// own font-size/font-weight class (no tailwind-merge in this project,
	// same hazard as IconButton.svelte's `background`, and CSS source order
	// - not the order classes appear in this string - decides the winner,
	// so appending `className}` last is not enough on its own; measured
	// against the compiled output, the caller's classes lost). Matched
	// against the closed set of this design system's font-size tokens
	// (`app.css`) and Tailwind's font-weight keywords specifically, rather
	// than a bare `text-`/`font-` prefix: `text-` also names colour
	// utilities (UserRow.svelte's role pill passes `text-accent-foreground`
	// / `text-text-muted`, which must NOT suppress the default size here)
	// and `font-` also names this app's font-family tokens (`font-display`,
	// `font-sans`).
	const FONT_SIZE_RE =
		/\btext-(label|micro|caption|body-sm|body-lg|body|card|heading-lg|heading|display-sm|display-md|display-lg|display-xl)\b/;
	const FONT_WEIGHT_RE =
		/\bfont-(thin|extralight|light|normal|medium|semibold|bold|extrabold|black)\b/;
	const size = $derived(FONT_SIZE_RE.test(className) ? '' : 'text-caption');
	const weight = $derived(FONT_WEIGHT_RE.test(className) ? '' : 'font-semibold');
</script>

<BitsSelect.Root type="single" bind:value {disabled} onValueChange={(next) => onchange?.(next)}>
	<BitsSelect.Trigger
		aria-label={label}
		class="inline-flex h-8 items-center gap-1.5 rounded-pill px-3 {size} {weight} transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-50 {className}"
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
