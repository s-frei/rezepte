/**
 * Stable handles on a person's row in the people list. A role change moves
 * the row to another group, which re-creates it and drops whatever had
 * focus inside it; these let the list put focus back on the same control in
 * the new row, found by the person's id rather than by an element reference
 * that died with the old one.
 */

/** The `id` of the row for this person. */
export function personRowId(id: string): string {
	return `person-${id}`;
}

/** Focuses the role select in this person's row, if it is on the page. */
export function focusRoleControl(id: string): void {
	document
		.getElementById(personRowId(id))
		?.querySelector<HTMLElement>('[data-role-control] button')
		?.focus();
}

/** The `id` of the "more actions" menu trigger in this person's row. */
export function menuTriggerId(id: string): string {
	return `person-${id}-menu`;
}

/** Focuses the "more actions" menu trigger in this person's row, if it is on the page. */
export function focusMenuTrigger(id: string): void {
	document.getElementById(menuTriggerId(id))?.focus();
}
