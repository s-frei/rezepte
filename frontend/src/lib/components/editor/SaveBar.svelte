<script lang="ts">
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';

	let {
		dirty = false,
		saving = false,
		oncancel,
		onsave
	}: {
		/** Shows the "unsaved changes" marker. */
		dirty?: boolean;
		/** Disables both buttons and switches the primary label while a save runs. */
		saving?: boolean;
		oncancel: () => void;
		onsave: () => void;
	} = $props();
</script>

<!--
	Sticks above the floating bottom nav on mobile (the same 96px offset the
	detail page's mobile CTA uses) and to the viewport bottom on desktop. The
	negative margins let it span the app shell's horizontal padding.
-->
<div
	class="sticky bottom-24 z-20 -mx-5 mt-8 flex h-[72px] items-center gap-3 border-t border-border bg-surface px-5 md:bottom-0 md:-mx-8 md:px-8"
>
	{#if dirty}
		<span class="mr-auto flex items-center gap-2 text-caption text-text-muted">
			<span aria-hidden="true" class="size-2 rounded-pill bg-current"></span>
			{m.editor_unsaved_changes()}
		</span>
	{/if}
	<Button variant="ghost" disabled={saving} onclick={oncancel} class={dirty ? '' : 'ml-auto'}>
		{m.common_cancel()}
	</Button>
	<Button variant="primary" disabled={saving} onclick={onsave}>
		{saving ? m.common_saving() : m.common_save()}
	</Button>
</div>
