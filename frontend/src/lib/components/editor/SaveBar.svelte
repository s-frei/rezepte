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
	Mobile: pinned to the top of the screen, not the bottom. The software
	keyboard owns the bottom while the cook types a step, and the two floating
	layers there (nav plus save card) used to eat a third of the screen. The
	bar carries no title: the two buttons already say where you are, and a
	headline squeezed between them would only ever truncate. The two ways out
	sit at opposite edges, so neither is hit by accident.
-->
<div
	class="fixed inset-x-0 top-0 z-40 flex h-16 items-center gap-3 border-b border-border bg-background/90 px-5 backdrop-blur md:hidden"
>
	<Button variant="ghost" disabled={saving} onclick={oncancel}>
		{m.common_cancel()}
	</Button>
	{#if dirty}
		<span class="ml-auto flex items-center">
			<span aria-hidden="true" class="size-2 rounded-pill bg-primary"></span>
			<span class="sr-only">{m.editor_unsaved_changes()}</span>
		</span>
	{/if}
	<Button variant="primary" disabled={saving} onclick={onsave} class={dirty ? '' : 'ml-auto'}>
		{saving ? m.common_saving() : m.common_save()}
	</Button>
</div>

<!--
	Desktop: a sticky full-width bar on the viewport bottom; the negative
	margins let it span the app shell's padding.
-->
<div
	class="sticky bottom-0 -mx-8 mt-8 hidden h-[72px] items-center gap-3 border-t border-border bg-surface px-8 md:flex"
>
	{#if dirty}
		<span class="mr-auto flex items-center gap-2 text-caption text-text-muted">
			<span aria-hidden="true" class="size-2 rounded-pill bg-current"></span>
			<span>{m.editor_unsaved_changes()}</span>
		</span>
	{/if}
	<Button variant="ghost" disabled={saving} onclick={oncancel} class={dirty ? '' : 'ml-auto'}>
		{m.common_cancel()}
	</Button>
	<Button variant="primary" disabled={saving} onclick={onsave}>
		{saving ? m.common_saving() : m.common_save()}
	</Button>
</div>
