<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade } from 'svelte/transition';

	// The dim behind every dialog and sheet, fading in and out with it.
	let { clipPath }: { clipPath?: string } = $props();
</script>

<Dialog.Overlay forceMount>
	{#snippet child({ props, open })}
		{#if open}
			<div
				{...props}
				class="fixed inset-0 z-40 bg-overlay"
				style:clip-path={clipPath}
				transition:fade={{ duration: prefersReducedMotion.current ? 0 : 200 }}
			></div>
		{/if}
	{/snippet}
</Dialog.Overlay>
