<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Label } from 'bits-ui';
	import type { HTMLInputAttributes } from 'svelte/elements';
	import RequiredMark, { requiredState } from './RequiredMark.svelte';

	let {
		id,
		label,
		error = null,
		hint,
		prefix,
		suffix,
		counter,
		aside,
		required = false,
		floating = false,
		value = $bindable(''),
		class: className = '',
		...rest
	}: {
		id: string;
		label: string;
		error?: string | null;
		/** Helper text below the field, hidden while an error takes its place. */
		hint?: string;
		/**
		 * Words rendered inside the field, left, before whatever is typed - the
		 * start of a sentence the value completes ("Adapted from …").
		 */
		prefix?: string;
		/** Unit rendered inside the field, right-aligned (e.g. "Min"). */
		suffix?: string;
		/**
		 * Character limit to count towards, shown inside the field once the
		 * value approaches it. Pass the same number as `maxlength`.
		 */
		counter?: number;
		/** Small text or links at the end of the label's line ("set · remove"). */
		aside?: Snippet;
		/**
		 * Marks the field as one the form cannot be saved without: a star on
		 * the label and a stroke in the margin while it is empty. Deliberately
		 * not the native attribute, which would have the browser stop the
		 * submit with its own bubble instead of the form's error on save.
		 */
		required?: boolean;
		/**
		 * The sign-in pages' field (AuthScene): the label sits inside it and
		 * floats up on focus or once it holds a value, and a field marked
		 * `aria-invalid` (the `error` prop, or the attribute passed straight
		 * through for one message under several fields) has its outline
		 * traced in red. Its moves wait for `data-ready` on an ancestor, so
		 * whatever is focused or filled when the page opens is simply there.
		 */
		floating?: boolean;
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

	const status = $derived(requiredState(value.trim() !== '', !!error));

	// Measured, not fixed: the prefix is a word in the reader's language,
	// "Nach" in one and "Adapted from" in another, and typed text must never
	// start underneath it.
	let prefixWidth = $state(0);

	const classes = $derived(
		`${floating ? 'h-14.5 border-border pt-5.5 pb-1.5' : `h-11 transition ${error ? 'border-[1.5px] border-destructive' : 'border-border'}`} w-full rounded-md border bg-surface-elevated px-4 text-body outline-none focus:border-primary ${suffix ? 'pr-12' : ''} ${counter !== undefined ? 'pr-16' : ''} ${className}`
	);

	// Prefix and suffix are part of what the field means ("30" is 30 minutes,
	// "Grandma's cookbook" is what it was adapted from), so they are described rather
	// than hidden from assistive tech.
	const describedBy = $derived(
		[
			prefix ? `${id}-prefix` : null,
			error ? `${id}-error` : hint ? `${id}-hint` : null,
			suffix ? `${id}-suffix` : null
		]
			.filter(Boolean)
			.join(' ') || undefined
	);
</script>

<div class={['space-y-1.5', { floating }]}>
	<!-- The star stands beside the label, not inside it: the label's text is
	     what `getByLabel`, password managers and autofill match the field
	     by, and "Password *" is not the field's name. -->
	{#snippet name()}
		<Label.Root for={id}>{label}</Label.Root>
		{#if required}
			<RequiredMark state={status} />
		{/if}
	{/snippet}
	{#if !floating}
		<div class="flex items-baseline gap-1 text-caption font-semibold">
			{@render name()}
			{#if aside}
				<span class="ml-auto font-normal text-text-muted">{@render aside()}</span>
			{/if}
		</div>
	{/if}
	<!-- Wraps only the field, so an absolutely positioned suffix lines up
	     with it regardless of the label above or the error below. -->
	<div class="relative">
		<input
			{id}
			bind:value
			aria-invalid={error ? 'true' : undefined}
			aria-required={required ? 'true' : undefined}
			aria-describedby={describedBy}
			class={classes}
			style:padding-left={prefix ? `calc(${prefixWidth}px + 1.5rem)` : undefined}
			placeholder={floating ? ' ' : undefined}
			{...rest}
		/>
		{#if floating}
			<!-- After the field, so its focus and its value can move it. The
			     label stays the field's real `<label for>`: it is what
			     `getByLabel`, password managers and autofill go by. -->
			<span
				data-float-label
				class="pointer-events-none absolute top-4.5 left-4.25 flex origin-top-left items-baseline gap-1 text-body text-text-muted"
			>
				{@render name()}
			</span>
			<!-- The outline drawn from the top-left corner round on an error;
			     it lies on the field's own border, which turns red as it closes. -->
			<svg
				aria-hidden="true"
				class="pointer-events-none absolute inset-[0.75px] size-[calc(100%-1.5px)] overflow-visible"
			>
				<rect width="100%" height="100%" pathLength="100" />
			</svg>
		{/if}
		{#if required}
			<!--
				A pencil tick in the margin, the way a cook marks the line still to
				do. It says one thing - this is empty and needs filling - so it
				leaves once there is content, and also once a save has failed:
				the red border and message say it louder then, and a third red
				mark would only shout. It sits outside the field, so it needs
				free space on the left; the containers that hold required fields
				all have at least 24px of padding there.
			-->
			<span
				aria-hidden="true"
				data-required-stroke
				data-state={status}
				class="pointer-events-none absolute inset-y-2 -left-2.5 w-0.75 rounded-pill bg-primary transition-opacity motion-reduce:transition-none {status ===
				'empty'
					? 'opacity-100'
					: 'opacity-0'}"
			></span>
		{/if}
		{#if prefix}
			<span
				id="{id}-prefix"
				bind:offsetWidth={prefixWidth}
				class="pointer-events-none absolute inset-y-0 left-4 flex items-center font-display text-body-sm text-text-muted italic"
				>{prefix}</span
			>
		{/if}
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
		<p id="{id}-error" class="text-micro font-medium text-destructive motion-safe:animate-error-in">
			{error}
		</p>
	{:else if hint}
		<p id="{id}-hint" class="text-micro text-text-muted">{hint}</p>
	{/if}
</div>

<style>
	/* The floating field. A value the browser filled in itself matches no
	   `:placeholder-shown`, so autofill is asked on its own: one unsupported
	   pseudo-class in a list would drop the whole rule. */
	.floating input:focus ~ [data-float-label],
	.floating input:not(:placeholder-shown) ~ [data-float-label] {
		translate: 0 -10px;
		scale: 0.78;
		font-weight: 600;
	}

	.floating input:-webkit-autofill ~ [data-float-label] {
		translate: 0 -10px;
		scale: 0.78;
		font-weight: 600;
	}

	.floating input:focus {
		box-shadow: 0 0 0 4px color-mix(in oklab, var(--color-primary) 14%, transparent);
	}

	.floating input:focus ~ [data-float-label] {
		color: var(--color-primary);
	}

	.floating rect {
		rx: calc(var(--radius-md) - 0.75px);
		fill: none;
		stroke: var(--color-destructive);
		stroke-width: 1.5px;
		stroke-dasharray: 100;
		stroke-dashoffset: 100;
	}

	/* A wrong field is red all through: the traced outline, then its border,
	   and refocused its ring and its label too. */
	.floating:has(input[aria-invalid='true']) rect {
		stroke-dashoffset: 0;
	}

	.floating input[aria-invalid='true'] {
		border-color: var(--color-destructive);
	}

	.floating input[aria-invalid='true']:focus {
		box-shadow: 0 0 0 4px color-mix(in oklab, var(--color-destructive) 14%, transparent);
	}

	.floating input[aria-invalid='true']:focus ~ [data-float-label] {
		color: var(--color-destructive);
	}

	/* Only the way in moves (and only once the page is ready): the trace
	   runs 320ms, the border turns as it closes, and a second wrong field
	   follows 60ms later. Typing takes the error away at once. */
	@media (prefers-reduced-motion: no-preference) {
		:global([data-ready]) .floating [data-float-label] {
			transition:
				translate 300ms var(--ease-spring),
				scale 300ms var(--ease-spring),
				color 160ms;
		}

		:global([data-ready]) .floating input {
			transition:
				border-color 160ms,
				box-shadow 220ms var(--ease-glide);
		}

		:global([data-ready]) .floating:has(input[aria-invalid='true']) rect {
			transition: stroke-dashoffset 320ms cubic-bezier(0.5, 0, 0.3, 1);
		}

		:global([data-ready]) .floating input[aria-invalid='true'] {
			transition:
				border-color 0ms 320ms,
				box-shadow 220ms var(--ease-glide);
		}

		:global([data-ready] .floating) ~ .floating:has(input[aria-invalid='true']) rect {
			transition-delay: 60ms;
		}

		:global([data-ready] .floating) ~ .floating input[aria-invalid='true'] {
			transition-delay: 380ms, 0ms;
		}
	}
</style>
