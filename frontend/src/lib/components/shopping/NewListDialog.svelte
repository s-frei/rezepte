<script lang="ts">
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { MediaQuery } from 'svelte/reactivity';
	import { goto } from '$app/navigation';
	import { isSignedOut } from '#lib/api/client.js';
	import { createShoppingList } from '#lib/api/shopping.js';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import { listHref } from '#lib/shopping/href.js';
	import { m } from '#lib/paraglide/messages.js';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	// The same breakpoint as the bottom nav: a phone gets a sheet.
	const phone = new MediaQuery('max-width: 767.98px');

	let name = $state('');
	let saving = $state(false);

	$effect(() => {
		if (open) name = '';
	});

	async function create(event: SubmitEvent) {
		event.preventDefault();
		const trimmed = name.trim();
		if (!trimmed || saving) return;
		saving = true;
		try {
			const list = await createShoppingList(trimmed);
			open = false;
			await goto(listHref(list.id));
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.lists_create_error());
		} finally {
			saving = false;
		}
	}

	// The sheet would focus its handle; the name field is what it is opened for.
	function focusField(event: Event) {
		event.preventDefault();
		document.getElementById('new-list-name')?.focus();
	}
</script>

{#snippet form()}
	<Dialog.Title class="font-display text-heading font-medium">{m.lists_new()}</Dialog.Title>
	<form onsubmit={create} class="mt-4 space-y-5">
		<Input
			id="new-list-name"
			label={m.lists_new_name_label()}
			placeholder={m.lists_new_name_placeholder()}
			autocomplete="off"
			maxlength={80}
			bind:value={name}
		/>
		<DialogActions>
			<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
			<Button type="submit" busy={saving} disabled={!name.trim()}>{m.lists_new_create()}</Button>
		</DialogActions>
	</form>
{/snippet}

{#if phone.current}
	<BottomSheet bind:open closeLabel={m.common_cancel()} onOpenAutoFocus={focusField}>
		<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">{@render form()}</div>
	</BottomSheet>
{:else}
	<BaseDialog bind:open>{@render form()}</BaseDialog>
{/if}
