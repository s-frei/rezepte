<script lang="ts">
	import Import from '@lucide/svelte/icons/import';
	import PencilLine from '@lucide/svelte/icons/pencil-line';
	import { Dialog } from 'bits-ui';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import BottomSheet from '$lib/components/ui/BottomSheet.svelte';
	import { importDialog } from '$lib/import.svelte';
	import { m } from '$lib/paraglide/messages';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	function write() {
		open = false;
		void goto(resolve('/recipes/new'));
	}

	function startImport() {
		open = false;
		importDialog.open = true;
	}

	const row =
		'flex w-full items-center gap-3 border-b border-border py-3.5 text-left last:border-b-0 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary';
	const tile =
		'flex size-10 shrink-0 items-center justify-center rounded-md bg-accent text-accent-foreground';
</script>

<!-- The phone's "+": two ways to a new recipe, writing one or importing one. -->
<BottomSheet bind:open closeLabel={m.common_close()}>
	<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">
		<Dialog.Title class="font-display text-heading font-medium">{m.new_sheet_title()}</Dialog.Title>
		<div class="mt-2">
			<button type="button" onclick={write} class={row}>
				<span class={tile} aria-hidden="true"><PencilLine class="size-5" /></span>
				<span>
					<span class="block font-semibold">{m.new_sheet_write()}</span>
					<span class="block text-body-sm text-text-muted">{m.new_sheet_write_hint()}</span>
				</span>
			</button>
			<button type="button" onclick={startImport} class={row}>
				<span class={tile} aria-hidden="true"><Import class="size-5" /></span>
				<span>
					<span class="block font-semibold">{m.new_sheet_import()}</span>
					<span class="block text-body-sm text-text-muted">{m.new_sheet_import_hint()}</span>
				</span>
			</button>
		</div>
	</div>
</BottomSheet>
