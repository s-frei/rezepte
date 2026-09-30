<script lang="ts">
	import { Checkbox } from 'bits-ui';
	import Check from '@lucide/svelte/icons/check';

	let {
		checked = $bindable(false),
		label,
		disabled = false
	}: { checked?: boolean; label: string; disabled?: boolean } = $props();
</script>

<Checkbox.Root
	bind:checked
	{disabled}
	aria-label={label}
	class="flex size-5 shrink-0 items-center justify-center rounded-word border-[1.5px] border-text-muted bg-surface-elevated transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-40 data-[state=checked]:border-primary data-[state=checked]:bg-primary"
>
	{#snippet children({ checked })}
		{#if checked}
			<Check class="size-3.5 text-primary-foreground" strokeWidth={3} aria-hidden="true" />
		{/if}
	{/snippet}
</Checkbox.Root>
