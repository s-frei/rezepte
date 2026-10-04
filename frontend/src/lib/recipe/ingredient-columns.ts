/** A run of one group's ingredients, `from` inclusive to `to` exclusive, inside one column. */
export type ColumnPart = { group: number; from: number; to: number; heading: boolean };

type Group = { name: string | null; ingredients: readonly unknown[] };

/** Below this many rows, group names counted, the list stays in one column. */
export const MIN_ROWS = 6;

const whole = (groups: readonly Group[], g: number): ColumnPart => ({
	group: g,
	from: 0,
	to: groups[g].ingredients.length,
	heading: !!groups[g].name
});

/**
 * Splits an ingredient list into a left and a right column for the tablet
 * layout, counting a group name as one row. A split between groups wins when
 * it leaves the sides at most 2 rows apart; otherwise the list breaks inside a
 * group where the sides come out most even, and the continuation in the right
 * column repeats no heading. On a tie the left column takes the extra row.
 * A list of fewer than `minRows` rows stays whole in the left column.
 */
export function splitColumns(
	groups: readonly Group[],
	minRows = MIN_ROWS
): [ColumnPart[], ColumnPart[]] {
	const rows = groups.map((group) => (group.name ? 1 : 0) + group.ingredients.length);
	const total = rows.reduce((sum, n) => sum + n, 0);
	// A short list reads better whole.
	if (total < minRows) return [groups.map((_, g) => whole(groups, g)), []];
	let best = { group: groups.length, at: 0, diff: Infinity, leftShort: true };
	const consider = (group: number, at: number, left: number) => {
		const diff = Math.abs(total - 2 * left);
		const leftShort = 2 * left < total;
		if (diff < best.diff || (diff === best.diff && best.leftShort && !leftShort)) {
			best = { group, at, diff, leftShort };
		}
	};

	let before = 0;
	for (let g = 1; g < groups.length; g++) consider(g, 0, (before += rows[g - 1]));
	if (best.diff > 2) {
		before = 0;
		groups.forEach((group, g) => {
			const heading = group.name ? 1 : 0;
			for (let i = 1; i < group.ingredients.length; i++) consider(g, i, before + heading + i);
			before += rows[g];
		});
	}

	const left: ColumnPart[] = [];
	const right: ColumnPart[] = [];
	groups.forEach((_, g) => {
		const part = whole(groups, g);
		if (g < best.group) left.push(part);
		else if (g > best.group || best.at === 0) right.push(part);
		else {
			left.push({ ...part, to: best.at });
			right.push({ ...part, from: best.at, heading: false });
		}
	});
	return [left, right];
}
