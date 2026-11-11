<script lang="ts">
	import { Dialog } from 'bits-ui';
	import Button from '#lib/components/ui/Button.svelte';
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import ResponsiveDialog from '#lib/components/ui/ResponsiveDialog.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { narrow } from '#lib/ui/narrow.js';

	let {
		open = $bindable(false),
		person,
		onconfirm
	}: {
		open?: boolean;
		person: { id: string; displayName: string; color: string; avatarId: string | null } | null;
		onconfirm: () => void;
	} = $props();

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
			class="mt-4 flex items-center gap-3 rounded-2xl p-3 {narrow.current
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

<ResponsiveDialog
	bind:open
	closeLabel={m.common_cancel()}
	onCloseAutoFocus={closeFocus}
	bodyClass="pb-8"
	{body}
/>
