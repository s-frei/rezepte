<script lang="ts">
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import { Dialog } from 'bits-ui';
	import { MediaQuery } from 'svelte/reactivity';
	import type { ShoppingList } from '#lib/api/shopping.js';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import PeopleList from './PeopleList.svelte';

	let {
		open = $bindable(false),
		list,
		onshare
	}: {
		open?: boolean;
		list: ShoppingList;
		/** Given for the creator only, who alone may share. */
		onshare?: () => void;
	} = $props();

	// The side card's "Shared with" for screens without the side column: a tap on the faces opens it.
	const phone = new MediaQuery('max-width: 767.98px');

	function share() {
		open = false;
		onshare?.();
	}
</script>

{#snippet body()}
	<Dialog.Title class="font-display text-heading-lg font-medium"
		>{m.list_side_people()}</Dialog.Title
	>
	<div class="mt-2"><PeopleList {list} /></div>
{/snippet}

{#snippet footer()}
	{#if onshare}
		<Button variant="secondary" onclick={share} class="grow md:grow-0">{m.list_share()}</Button>
	{/if}
	<Button onclick={() => (open = false)} class="grow md:grow-0">{m.item_sheet_done()}</Button>
{/snippet}

{#if phone.current}
	<BottomSheet bind:open closeLabel={m.item_sheet_done()}>
		<div data-sheet-body class="min-h-0 flex-1 overflow-y-auto px-5 pb-4">{@render body()}</div>
		<div class="flex flex-wrap gap-3 border-t border-border px-5 pt-3.5 pb-5">
			{@render footer()}
		</div>
	</BottomSheet>
{:else}
	<BaseDialog bind:open>
		{@render body()}
		<DialogActions class="mt-6 border-t border-border pt-4">{@render footer()}</DialogActions>
	</BaseDialog>
{/if}
