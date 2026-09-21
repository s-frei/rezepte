<script lang="ts">
	import { Label } from 'bits-ui';
	import type { HTMLInputAttributes } from 'svelte/elements';

	let {
		id,
		label,
		error = null,
		hint,
		suffix,
		counter,
		value = $bindable(''),
		class: className = '',
		...rest
	}: {
		id: string;
		label: string;
		error?: string | null;
		/** Helper text below the field, hidden while an error takes its place. */
		hint?: string;
		/** Unit rendered inside the field, right-aligned (e.g. "Min"). */
		suffix?: string;
		/**
		 * Character limit to count towards, shown inside the field once the
		 * value approaches it. Pass the same number as `maxlength`.
		 */
		counter?: number;
		value?: string;
		class?: string;
	} & Omit<HTMLInputAttributes, 'id' | 'value' | 'class'> = $props();

	// Code points, not UTF-16 units. The limit these count towards is counted
	// in runes on the server, and `.length` says 2 for one emoji - a counter
	// that disagrees with the limit it displays is worse than none.
	const counted = $derived(counter === undefined ? 0 : [...value].length);
	// Quiet until three quarters full. Most values sit nowhere near the
	// limit, and a number that never changes meaningfully is noise competing
	// with the text someone is actually typing.
	const showCounter = $derived(counter !== undefined && counted >= counter * 0.75);
	const atLimit = $derived(counter !== undefined && counted >= counter);
	// The padding below is reserved for the whole life of a counting field,
	// not only while the counter shows: text that reflowed at the 48th
	// character would draw more attention than the counter appearing does.

	const classes = $derived(
		`h-11 w-full rounded-md border bg-surface-elevated px-4 text-body transition outline-none focus:border-primary ${error ? 'border-[1.5px] border-destructive' : 'border-border'} ${suffix ? 'pr-12' : ''} ${counter !== undefined ? 'pr-16' : ''} ${className}`
	);

	// The suffix is part of what the field means ("30" is 30 minutes), so it
	// is described rather than hidden from assistive tech.
	const describedBy = $derived(
		[error ? `${id}-error` : hint ? `${id}-hint` : null, suffix ? `${id}-suffix` : null]
			.filter(Boolean)
			.join(' ') || undefined
	);
</script>

<div class="space-y-1.5">
	<Label.Root for={id} class="text-caption font-semibold">{label}</Label.Root>
	<!-- Wraps only the field, so an absolutely positioned suffix lines up
	     with it regardless of the label above or the error below. -->
	<div class="relative">
		<input
			{id}
			bind:value
			aria-invalid={error ? 'true' : undefined}
			aria-describedby={describedBy}
			class={classes}
			{...rest}
		/>
		{#if suffix}
			<span
				id="{id}-suffix"
				class="pointer-events-none absolute inset-y-0 right-4 flex items-center text-caption text-text-muted"
			>
				{suffix}
			</span>
		{/if}
		<!--
			Hidden from assistive tech on purpose: `maxlength` already tells a
			screen reader what the field accepts, and a count that changed with
			every keystroke would either say nothing (not a live region) or say
			far too much (one announcement per character).

			A full field is not an error, so it does not turn destructive - a
			name of exactly the maximum length is valid, and a red counter
			greeting someone on page load would say otherwise. It loses the
			muted tone instead, which is enough to explain why typing stopped.
		-->
		{#if showCounter}
			<span
				aria-hidden="true"
				class="pointer-events-none absolute inset-y-0 right-4 flex items-center text-micro tabular-nums {atLimit
					? 'font-semibold text-text'
					: 'text-text-muted'}"
			>
				{counted}/{counter}
			</span>
		{/if}
	</div>
	{#if error}
		<p id="{id}-error" class="text-micro font-medium text-destructive">{error}</p>
	{:else if hint}
		<p id="{id}-hint" class="text-micro text-text-muted">{hint}</p>
	{/if}
</div>
