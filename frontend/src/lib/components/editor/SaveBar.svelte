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
	Mobile: a floating card fixed 16px above the bottom nav (nav top edge is
	80px from the viewport bottom; the detail page's cook-mode CTA uses the
	same 96px offset). Desktop: a sticky full-width bar on the viewport
	bottom; the negative margins let it span the app shell's padding.
-->
<div
	class="fixed inset-x-4 bottom-24 z-20 flex h-16 items-center gap-3 rounded-2xl bg-surface px-4 shadow-dialog md:sticky md:inset-x-auto md:bottom-0 md:-mx-8 md:mt-8 md:h-[72px] md:rounded-none md:border-t md:border-border md:px-8 md:shadow-none"
>
	{#if dirty}
		<!-- The card is too narrow on phones for the caption next to two
		     buttons, so only the dot stays visible there; the text remains for
		     assistive tech and desktop. -->
		<span class="mr-auto flex items-center gap-2 text-caption text-text-muted">
			<span aria-hidden="true" class="size-2 rounded-pill bg-current"></span>
			<span class="sr-only md:not-sr-only">{m.editor_unsaved_changes()}</span>
		</span>
	{/if}
	<Button variant="ghost" disabled={saving} onclick={oncancel} class={dirty ? '' : 'ml-auto'}>
		{m.common_cancel()}
	</Button>
	<Button variant="primary" disabled={saving} onclick={onsave}>
		{saving ? m.common_saving() : m.common_save()}
	</Button>
</div>
