/** A stamp's measured widths: whole, and without its detail ("· 10 offen", "· Mo"). */
export type StampWidth = { full: number; bare: number };

/**
 * What a row of stamps shows: the first `shown`, all whole or all `bare`
 * (detail dropped), the rest behind a "+n" on the last one's line; or, when
 * not even the first stamp fits beside its "+n", one `summary` pill
 * ("3 Listen") that holds them all.
 */
export type StampFit = { shown: number; bare: boolean; summary: boolean };

/**
 * The one fit rule for every row of stamps. Laid out in order, line by line
 * as flex-wrap does, a stamp that does not fit on a line starts the next.
 * Whole stamps when they all fit; else the row drops every detail at once if
 * that shows more of them; the rest go behind a "+n" on the last shown
 * stamp's line. A name is never cut: when not even the first bare stamp
 * fits beside its "+n", the row is one summary.
 */
export function fitStamps(
	stamps: StampWidth[],
	row: number,
	lines: number,
	gap: number,
	more: number
): StampFit {
	const most = (bare: boolean) => {
		const widths = stamps.map((s) => (bare ? s.bare : s.full));
		for (let shown = widths.length; shown >= 1; shown--)
			if (fits(widths.slice(0, shown), row, lines, gap, shown < widths.length ? more : 0))
				return shown;
		return 0;
	};
	const whole = most(false);
	if (whole === stamps.length) return { shown: whole, bare: false, summary: false };
	const bare = most(true);
	if (bare > whole) return { shown: bare, bare: true, summary: false };
	if (whole > 0) return { shown: whole, bare: false, summary: false };
	return { shown: 0, bare: false, summary: stamps.length > 0 };
}

/** Whether the widths fit in `lines` lines of `row` px, `more` riding with the last one. */
function fits(widths: number[], row: number, lines: number, gap: number, more: number): boolean {
	let line = 1;
	let used = 0;
	for (const [i, w] of widths.entries()) {
		const width = w + (i === widths.length - 1 && more ? gap + more : 0);
		if (width > row) return false;
		if (used && used + gap + width > row) {
			if (++line > lines) return false;
			used = 0;
		}
		used = (used ? used + gap : 0) + width;
	}
	return true;
}
