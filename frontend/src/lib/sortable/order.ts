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
 * Where Alt+`key` moves the item at `index`: Left/Right by one, Up/Down by a
 * row of `columns`, clamped to the list. `null` for any other key, or when
 * the item already stands at that end.
 */
export function keyTarget(
	index: number,
	length: number,
	key: string,
	columns: number
): number | null {
	const steps: Record<string, number> = {
		ArrowLeft: -1,
		ArrowRight: 1,
		ArrowUp: -columns,
		ArrowDown: columns
	};
	const step = steps[key];
	if (step === undefined) return null;
	const to = Math.max(0, Math.min(index + step, length - 1));
	return to === index ? null : to;
}
