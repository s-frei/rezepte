<script lang="ts">
	import type { Snippet } from 'svelte';
	import Check from 'lucide-svelte/icons/check';
	import ChevronDown from 'lucide-svelte/icons/chevron-down';
	import { Select as BitsSelect } from 'bits-ui';

	/**
	 * `hint` is a second, muted word on the same row of the open list - a
	 * language's endonym beside its translated name, say. It never reaches
	 * the trigger: the trigger has to stay one glance wide, and the hint is
	 * there to help someone *find* the entry, not to describe the choice
	 * once it is made.
	 */
	type Option = { value: string; label: string; hint?: string };

	let {
		value = $bindable(''),
		options,
		label,
		disabled = false,
		variant = 'field',
		icon,
		onchange,
		class: className = ''
	}: {
		value?: string;
		options: Option[];
		/** Accessible name of the trigger. */
		label: string;
		disabled?: boolean;
		/**
		 * Trigger shape. `field` is a bordered control the height of `Input`,
		 * for a select that sits among form fields; `pill` is the compact
		 * chip the members table uses in a row. It decides shape only - never
		 * the background, see `class`.
		 */
		variant?: 'field' | 'pill';
		/**
		 * Optional mark ahead of the label, for a select whose subject is not
		 * obvious from the chosen value alone - the overview's sort control
		 * reads "Zuletzt geändert", which names an order without saying that
		 * ordering is what it does. It sits where FilterPanel's trigger keeps
		 * its funnel, so the two controls read as a pair.
		 */
		icon?: Snippet;
		/** Fires when the user picks an option (not on programmatic changes). */
		onchange?: (value: string) => void;
		/**
		 * Extra trigger classes. **The ground belongs here, not in the
		 * variant.** A control sits one step above whatever it lies on, and
		 * that differs by place: `bg-surface-elevated` inside a dialog or a
		 * panel, `bg-surface` straight on the page. A default background
		 * would also collide with the caller's - there is no tailwind-merge
		 * here, so CSS source order would decide the winner.
		 */
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
	const shape = $derived(
		variant === 'pill' ? 'h-8 rounded-pill px-3' : 'h-11 rounded-md border border-border px-4'
	);
	const size = $derived(
		FONT_SIZE_RE.test(className) ? '' : variant === 'pill' ? 'text-caption' : 'text-body-sm'
	);
	const weight = $derived(
		FONT_WEIGHT_RE.test(className) ? '' : variant === 'pill' ? 'font-semibold' : 'font-medium'
	);
</script>

<BitsSelect.Root type="single" bind:value {disabled} onValueChange={(next) => onchange?.(next)}>
	<BitsSelect.Trigger
		aria-label={label}
		class="inline-flex items-center gap-1.5 {shape} {size} {weight} transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-50 {className}"
	>
		{#if icon}{@render icon()}{/if}
		{selectedLabel}
		<ChevronDown class="size-3.5 shrink-0" aria-hidden="true" />
	</BitsSelect.Trigger>
	<BitsSelect.Portal>
		<!--
			align="start" rather than the floating default: a trigger stretched
			by its container (the create-token form makes it form-wide) would
			otherwise centre the list on the full width while the visible label
			sits at the left edge, putting the list beside the control it
			belongs to. Aligning on the start edge is what a select is expected
			to do at any trigger width.

			bg-surface-elevated with a border, not bg-surface: inside a dialog
			the card is already bg-surface, so a list in the same colour has no
			visible edge, and shadow-dialog does not read against the dark
			scheme's surfaces.

			The width floor is the trigger's own width, which bits-ui exposes
			as --bits-floating-anchor-width, rather than a round number: the
			list is never narrower than the control it belongs to, and
			otherwise as wide as its longest entry. A fixed floor gets both
			ends wrong - too wide under the members table's compact role chip,
			too narrow under a form-wide trigger.
		-->
		<BitsSelect.Content
			preventScroll={false}
			align="start"
			sideOffset={6}
			class="z-50 min-w-[var(--bits-floating-anchor-width)] rounded-2xl border border-border bg-surface-elevated p-1.5 shadow-dialog"
		>
			{#each options as option (option.value)}
				<BitsSelect.Item
					value={option.value}
					label={option.label}
					class="flex h-10 cursor-default items-center justify-between gap-8 rounded-sm px-3 text-body-sm text-text outline-none data-highlighted:bg-background"
				>
					{#snippet children({ selected })}
						<span class="flex min-w-0 items-baseline gap-2">
							<span class="truncate">{option.label}</span>
							{#if option.hint}
								<span class="truncate text-caption text-text-muted">{option.hint}</span>
							{/if}
						</span>
						{#if selected}
							<Check
								class="size-[18px] shrink-0 text-primary"
								strokeWidth={1.75}
								aria-hidden="true"
							/>
						{/if}
					{/snippet}
				</BitsSelect.Item>
			{/each}
		</BitsSelect.Content>
	</BitsSelect.Portal>
</BitsSelect.Root>
