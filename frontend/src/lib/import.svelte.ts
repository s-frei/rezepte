/** Whether the import dialog (desktop) or sheet (phone) is open; set from
 * the top bar, the empty overview, the phone's New sheet and the palette.
 * A `prefill` is put into the field, ready to read (see `openImport`). */
export const importDialog = $state({ open: false, prefill: '' });

/** Opens the dialog; a `prefill` waits in the field for the read button. */
export function openImport(prefill = ''): void {
	importDialog.prefill = prefill;
	importDialog.open = true;
}
