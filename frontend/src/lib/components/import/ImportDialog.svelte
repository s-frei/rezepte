<script lang="ts">
	import { MediaQuery } from 'svelte/reactivity';
	import { session } from '$lib/auth.svelte';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import BottomSheet from '$lib/components/ui/BottomSheet.svelte';
	import { importDialog } from '$lib/import.svelte';
	import { m } from '$lib/paraglide/messages';
	import { isAdminRole } from '$lib/roles';
	import ImportForm from './ImportForm.svelte';

	// The same breakpoint as the bottom nav: a phone gets a sheet.
	const phone = new MediaQuery('max-width: 767.98px');
	const admin = $derived(isAdminRole(session.user?.role));
	const close = () => (importDialog.open = false);

	// The sheet would focus its handle, its first control, while the field
	// is what someone opens it for; the dialog reaches the field on its own.
	function focusField(event: Event) {
		event.preventDefault();
		document.getElementById('import-field')?.focus();
	}
</script>

{#if phone.current}
	<BottomSheet
		bind:open={importDialog.open}
		closeLabel={m.import_cancel()}
		onOpenAutoFocus={focusField}
	>
		<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">
			<ImportForm ondone={close} {admin} />
		</div>
	</BottomSheet>
{:else}
	<BaseDialog bind:open={importDialog.open}>
		<ImportForm ondone={close} {admin} />
	</BaseDialog>
{/if}
