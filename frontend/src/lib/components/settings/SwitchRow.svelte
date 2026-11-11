<script lang="ts">
	import type { ClassValue } from 'svelte/elements';
	import Switch from '#lib/components/ui/Switch.svelte';

	// A setting's title and hint, with the switch that turns it on beside them.
	let {
		title,
		hint,
		checked,
		onchange,
		class: className
	}: {
		title: string;
		hint: string;
		checked: boolean;
		onchange: (checked: boolean) => void;
		class?: ClassValue;
	} = $props();
</script>

<div class={['flex items-start justify-between gap-4', className]}>
	<div class="min-w-0">
		<p class="text-body font-semibold">{title}</p>
		<p class="mt-1 max-w-[60ch] text-caption text-text-muted">{hint}</p>
	</div>
	<Switch {checked} label={title} {onchange} />
</div>
