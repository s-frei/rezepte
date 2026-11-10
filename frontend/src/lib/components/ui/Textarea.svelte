<script lang="ts">
	import { Label } from 'bits-ui';
	import FieldError from './FieldError.svelte';
	import type { HTMLTextareaAttributes } from 'svelte/elements';

	let {
		id,
		label,
		error = null,
		value = $bindable(''),
		class: className = '',
		...rest
	}: {
		id: string;
		label: string;
		error?: string | null;
		value?: string;
		class?: string;
	} & Omit<HTMLTextareaAttributes, 'id' | 'value' | 'class'> = $props();

	const classes = $derived(
		`min-h-11 w-full resize-y rounded-md border bg-surface-elevated px-4 py-2.5 text-input text-text placeholder:text-text-muted transition focus-visible:outline-2 focus-visible:outline-offset-2 ${error ? 'border-[1.5px] border-destructive focus:border-destructive focus-visible:outline-destructive' : 'border-border focus:border-primary focus-visible:outline-primary'} ${className}`
	);
</script>

<div class="space-y-1.5">
	<Label.Root for={id} class="text-caption font-semibold">{label}</Label.Root>
	<textarea
		{id}
		bind:value
		aria-invalid={error ? 'true' : undefined}
		aria-describedby={error ? `${id}-error` : undefined}
		class={classes}
		{...rest}></textarea>
	{#if error}
		<FieldError id="{id}-error" message={error} />
	{/if}
</div>
