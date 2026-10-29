<script lang="ts">
	import { Checkbox } from 'bits-ui';

	let {
		checked = $bindable(false),
		label,
		disabled = false
	}: { checked?: boolean; label: string; disabled?: boolean } = $props();

	// The tick draws itself only when this box was clicked: a box that loads
	// checked, or that "select all" checks, shows it at once.
	let picked = $state(false);
</script>

<Checkbox.Root
	bind:checked
	{disabled}
	aria-label={label}
	onCheckedChange={() => (picked = true)}
	class="flex size-5 shrink-0 items-center justify-center rounded-word border-[1.5px] border-text-muted bg-surface-elevated transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-40 data-[state=checked]:border-primary data-[state=checked]:bg-primary"
>
	{#snippet children({ checked })}
		{#if checked}
			<!-- Lucide's check, its path turned around so it draws from the short
			     stroke to the long one. -->
			<svg
				xmlns="http://www.w3.org/2000/svg"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="3"
				stroke-linecap="round"
				stroke-linejoin="round"
				aria-hidden="true"
				class="size-3.5 text-primary-foreground"
			>
				<path d="m4 12 5 5L20 6" pathLength="1" class={picked ? 'draw' : ''} />
			</svg>
		{/if}
	{/snippet}
</Checkbox.Root>

<style>
	path {
		stroke-dasharray: 1;
	}

	@media (prefers-reduced-motion: no-preference) {
		/* The way a hand ticks a box. */
		.draw {
			animation: draw 200ms var(--ease-glide) both;
		}
	}

	@keyframes draw {
		from {
			stroke-dashoffset: 1;
		}
		to {
			stroke-dashoffset: 0;
		}
	}
</style>
