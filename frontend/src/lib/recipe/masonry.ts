/**
 * Masonry placement for cards in DOM order: each card goes into the shortest
 * column, its `top` that column's height so far. Columns within `tolerance`
 * of the shortest count as equal and the leftmost wins, so near-equal cards
 * fill left to right instead of zigzagging over a pixel or two. Placing only
 * looks back, so appending cards ("Load more") never moves the earlier ones.
 */
export function placeCards(
	heights: number[],
	columns: number,
	gap: number,
	tolerance = 16
): { column: number; top: number }[] {
	const columnHeights = new Array<number>(columns).fill(0);
	return heights.map((height) => {
		const min = Math.min(...columnHeights);
		const column = columnHeights.findIndex((h) => h - min < tolerance);
		const top = columnHeights[column];
		columnHeights[column] = top + height + gap;
		return { column, top };
	});
}
