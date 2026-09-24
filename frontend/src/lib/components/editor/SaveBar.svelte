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

	Nothing else belongs in here. A line saying how many ingredient links a
	save will confirm lived here for a while, and it cost a row of a bar that
	is pinned over the form - permanently, because there is nearly always an
	open proposal while a recipe is being written. It says the same thing in
	the steps section, beside the proposals it is about, and costs nothing
	when there are none.
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
	Desktop: the foot of the section rail, under the nav entries, inside the
	rail's sticky block - so it scrolls with the rail and never lies over the
	form. Any bar that floats above a scrolling form hides a band of exactly
	what is being read; the rail is 200px wide and empty for the form's whole
	height, which is the only place on this page that can hold a control
	without taking anything away.

	Both buttons stack: 200px cannot hold them side by side, and a pill that
	breaks into two lines is worse than a column. "Speichern" goes on top,
	where the eye leaves the nav.

	The status line sits under the buttons, not above them, so its appearing
	never pushes them down under the pointer. A rule is all the separation
	the block needs - a card here would put a second surface in a rail whose
	entries sit straight on the page.
-->
<div
	class="hidden w-full flex-col gap-2 border-t border-border pt-4 md:flex"
	data-testid="save-actions"
>
	<Button variant="primary" disabled={saving} onclick={onsave} class="w-full">
		{saving ? m.common_saving() : m.common_save()}
	</Button>
	<Button variant="ghost" disabled={saving} onclick={oncancel} class="w-full">
		{m.common_cancel()}
	</Button>
	{#if dirty}
		<span class="mt-1 flex items-center justify-center gap-2 text-caption text-text-muted">
			<span aria-hidden="true" class="size-2 shrink-0 rounded-pill bg-current"></span>
			<span>{m.editor_unsaved_changes()}</span>
		</span>
	{/if}
</div>
