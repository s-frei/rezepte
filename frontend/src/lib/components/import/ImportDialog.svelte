<script lang="ts">
	import { session } from '#lib/auth.svelte.js';
	import ResponsiveDialog from '#lib/components/ui/ResponsiveDialog.svelte';
	import { importDialog } from '#lib/import.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { isAdminRole } from '#lib/roles.js';
	import ImportForm from './ImportForm.svelte';
	import { focusField } from './PasteForm.svelte';

	const admin = $derived(isAdminRole(session.user?.role));
</script>

<ResponsiveDialog
	bind:open={importDialog.open}
	closeLabel={m.import_cancel()}
	onOpenAutoFocus={focusField('import-field')}
>
	<ImportForm ondone={() => (importDialog.open = false)} {admin} />
</ResponsiveDialog>
