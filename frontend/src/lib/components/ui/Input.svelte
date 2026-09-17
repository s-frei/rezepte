<script lang="ts">
	import { Label } from 'bits-ui';
	import type { HTMLInputAttributes } from 'svelte/elements';

	let {
		id,
		label,
		error = null,
		suffix,
		value = $bindable(''),
		class: className = '',
		...rest
	}: {
		id: string;
		label: string;
		error?: string | null;
		/** Unit rendered inside the field, right-aligned (e.g. "Min"). */
		suffix?: string;
		value?: string;
		class?: string;
	} & Omit<HTMLInputAttributes, 'id' | 'value' | 'class'> = $props();

	const classes = $derived(
		`h-11 w-full rounded-md border bg-surface-elevated px-4 text-body transition outline-none focus:border-primary ${error ? 'border-[1.5px] border-destructive' : 'border-border'} ${suffix ? 'pr-12' : ''} ${className}`
	);

	// The suffix is part of what the field means ("30" is 30 minutes), so it
	// is described rather than hidden from assistive tech.
	const describedBy = $derived(
		[error ? `${id}-error` : null, suffix ? `${id}-suffix` : null].filter(Boolean).join(' ') ||
			undefined
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
	</div>
	{#if error}
		<p id="{id}-error" class="text-micro font-medium text-destructive">{error}</p>
	{/if}
</div>
