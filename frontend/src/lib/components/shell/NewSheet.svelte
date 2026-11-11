<script lang="ts">
	import Import from '@lucide/svelte/icons/import';
	import PencilLine from '@lucide/svelte/icons/pencil-line';
	import { Dialog } from 'bits-ui';
	import { flushSync } from 'svelte';
	import { afterNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import { importDialog } from '#lib/import.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { listImport, onDraft, startList } from '#lib/shopping/new-list.svelte.js';

	// On the lists the "+" makes a list, by the same two ways.
	let { open = $bindable(false), lists = false }: { open?: boolean; lists?: boolean } = $props();

	// Back and forward navigate without a tap in the sheet; it closes with
	// them, as the You sheet does.
	afterNavigate(({ from, to, type }) => {
		// A page rewriting its own query (the overview's filters, once its
		// list arrives) has not been left; anything else closes it.
		if (type === 'goto' && from?.url.pathname === to?.url.pathname) return;
		open = false;
	});

	// A new list's draft takes the keyboard (`startList`). While the sheet is
	// open its focus trap pulls the stand-in's focus back, and closing hands
	// focus to "+"; iOS drops the keyboard with either. So the sheet closes
	// first, synchronously and without that hand-back, still inside the tap.
	let handing = $state(false);
	function write() {
		if (lists && !onDraft()) {
			handing = true;
			open = false;
			flushSync();
			startList();
			return;
		}
		open = false;
		if (!lists) void goto(resolve('/recipes/new'));
	}

	function closeFocus(event: Event) {
		if (!handing) return;
		handing = false;
		event.preventDefault();
	}

	function startImport() {
		open = false;
		if (lists) listImport.open = true;
		else importDialog.open = true;
	}

	const row =
		'flex w-full items-center gap-3 border-b border-border py-3.5 text-left last:border-b-0 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary';
	const tile =
		'flex size-10 shrink-0 items-center justify-center rounded-md bg-accent text-accent-foreground';
</script>

<!-- The phone's "+": two ways to something new where you are, writing or
     importing a recipe, or a list. -->
<BottomSheet bind:open closeLabel={m.common_close()} onCloseAutoFocus={closeFocus}>
	<div data-sheet-body class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">
		<Dialog.Title class="font-display text-heading font-medium"
			>{lists ? m.lists_new() : m.new_sheet_title()}</Dialog.Title
		>
		<div class="mt-2">
			<button type="button" onclick={write} class={row}>
				<span class={tile} aria-hidden="true"><PencilLine class="size-5" /></span>
				<span>
					<span class="block font-semibold"
						>{lists ? m.new_sheet_list_write() : m.new_sheet_write()}</span
					>
					<span class="block text-body-sm text-text-muted"
						>{lists ? m.new_sheet_list_write_hint() : m.new_sheet_write_hint()}</span
					>
				</span>
			</button>
			<button type="button" onclick={startImport} class={row}>
				<span class={tile} aria-hidden="true"><Import class="size-5" /></span>
				<span>
					<span class="block font-semibold">{m.new_sheet_import()}</span>
					<span class="block text-body-sm text-text-muted"
						>{lists ? m.new_sheet_list_import_hint() : m.new_sheet_import_hint()}</span
					>
				</span>
			</button>
		</div>
	</div>
</BottomSheet>
