<script lang="ts">
	import { Label } from 'bits-ui';
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
		`min-h-11 w-full resize-y rounded-md border bg-surface-elevated px-4 py-2.5 text-body transition outline-none focus:border-primary ${error ? 'border-[1.5px] border-destructive' : 'border-border'} ${className}`
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
		<p id="{id}-error" class="text-micro font-medium text-destructive">{error}</p>
	{/if}
</div>
