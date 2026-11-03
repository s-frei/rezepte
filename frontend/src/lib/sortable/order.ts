// Pure list moves for the sortable: a drag or a key press says from and to,
// these say where everything else ends up.

/** A copy of `list` with the item at `from` moved to `to`, clamped to the list. */
export function moveItem<T>(list: readonly T[], from: number, to: number): T[] {
	const next = [...list];
	const [item] = next.splice(from, 1);
	next.splice(Math.max(0, Math.min(to, next.length)), 0, item);
	return next;
}

/**
 * Where Alt+`key` moves the item at `index`: Left and Up one place earlier,
 * Right and Down one place later, clamped to the list (at an end it stays).
 * `null` for any other key.
 */
export function keyTarget(index: number, length: number, key: string): number | null {
	const steps: Record<string, number> = { ArrowLeft: -1, ArrowUp: -1, ArrowRight: 1, ArrowDown: 1 };
	const step = steps[key];
	if (step === undefined) return null;
	return Math.max(0, Math.min(index + step, length - 1));
}
