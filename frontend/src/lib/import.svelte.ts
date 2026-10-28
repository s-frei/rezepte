/** Whether the import dialog (desktop) or sheet (phone) is open; set from
 * the top bar, the empty overview, the phone's New sheet and the palette.
 * A `prefill` is put into the field and read at once (see `openImport`). */
export const importDialog = $state({ open: false, prefill: '' });

/** Opens the dialog; with a `prefill` it starts reading that right away. */
export function openImport(prefill = ''): void {
	importDialog.prefill = prefill;
	importDialog.open = true;
}
