<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { MediaQuery } from 'svelte/reactivity';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let {
		open = $bindable(false),
		person,
		onconfirm
	}: {
		open?: boolean;
		person: { id: string; displayName: string; color: string; avatarId: string | null } | null;
		onconfirm: () => void;
	} = $props();

	// The same breakpoint as the bottom nav: a phone gets a sheet.
	const phone = new MediaQuery('max-width: 767.98px');

	// A confirmed remove takes the ×, the dialog's way back, with it: the
	// page places focus itself once the card is gone.
	let confirmed = false;

	function confirm() {
		confirmed = true;
		open = false;
		onconfirm();
	}

	function closeFocus(event: Event) {
		if (confirmed) event.preventDefault();
		confirmed = false;
	}
</script>

{#snippet body()}
	<Dialog.Title class="font-display text-heading font-medium"
		>{m.switch_remove_title()}</Dialog.Title
	>
	{#if person}
		<!-- The person as a small card: raised off the sheet's ground, or the dialog's surface. -->
		<div
			class="mt-4 flex items-center gap-3 rounded-2xl p-3 {phone.current
				? 'bg-surface'
				: 'bg-surface-elevated'}"
		>
			<PersonMark {person} size="lg" />
			<span
				class="line-clamp-2 min-w-0 font-display text-body-lg leading-tight font-medium wrap-anywhere"
				>{person.displayName}</span
			>
		</div>
	{/if}
	<Dialog.Description class="mt-3 text-body-sm text-text-muted"
		>{m.switch_remove_body()}</Dialog.Description
	>
	<DialogActions class="mt-6">
		<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
		<Button variant="destructive" onclick={confirm}>{m.switch_remove_confirm()}</Button>
	</DialogActions>
{/snippet}

{#if phone.current}
	<BottomSheet bind:open closeLabel={m.common_cancel()} onCloseAutoFocus={closeFocus}>
		<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">{@render body()}</div>
	</BottomSheet>
{:else}
	<BaseDialog bind:open onCloseAutoFocus={closeFocus}>{@render body()}</BaseDialog>
{/if}
