<script lang="ts">
	import Check from '@lucide/svelte/icons/check';

	let {
		checked,
		covered = false,
		size = 'sm'
	}: {
		checked: boolean;
		/** Ticked through another tick (the tick row's covered pill): an accent disc. */
		covered?: boolean;
		/** 22px on the shopping list, 24px in the tick row, 30px in the shopping mode. */
		size?: 'sm' | 'md' | 'lg';
	} = $props();

	const box = {
		sm: 'size-[22px] border-[1.5px]',
		md: 'size-6 border-[1.5px]',
		lg: 'size-[30px] border-2'
	};
</script>

<!-- The round tick of the shopping list and the tick row: a muted ring that
     becomes a primary disc with a check. The button around it carries the
     state. Only the check is aria-hidden: the shopping e2e counts a row's
     faces as its aria-hidden circles. -->
<span
	class="flex shrink-0 items-center justify-center rounded-full transition {box[size]} {covered
		? 'border-accent bg-accent text-accent-foreground'
		: checked
			? 'border-primary bg-primary text-primary-foreground'
			: 'border-text-muted bg-surface-elevated'}"
>
	{#if checked || covered}<Check
			class={size === 'lg' ? 'size-4' : 'size-3.5'}
			strokeWidth={3}
			aria-hidden="true"
		/>{/if}
</span>
