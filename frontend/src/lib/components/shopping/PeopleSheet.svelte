<script lang="ts">
	import { Dialog } from 'bits-ui';
	import type { ShoppingList } from '#lib/api/shopping.js';
	import ResponsiveDialog from '#lib/components/ui/ResponsiveDialog.svelte';
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

<ResponsiveDialog bind:open closeLabel={m.item_sheet_done()} {body} {footer} />
