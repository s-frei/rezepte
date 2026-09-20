/**
 * The scale behind the overview's time filter.
 *
 * The filter itself is a bound in minutes (`maxMinutes`, 0 = off) that the
 * URL and the API both speak, and the API takes any value from 0 to 1440.
 * The control that sets it offers a fixed list of stops instead, because a
 * cook thinks in quarter hours and then in hours, not in minutes: the steps
 * are even on screen while the minutes behind them grow coarser upwards.
 *
 * Everything that maps between the two lives here rather than in the
 * control, so `query.ts` can hold the URL to the same scale - a link is
 * then always a state the slider can show, and its label never names a
 * bound different from the one the grid is answering.
 */

/**
 * Every bound the filter can take, in the order the slider lays them out.
 * The last one is 0: no bound at all, the filter switched off. It sits at
 * the loose end of the scale because that is what it is - more time allowed
 * than any other stop, not a separate mode beside them.
 */
export const TIME_STOPS: readonly number[] = [15, 30, 45, 60, 90, 120, 180, 240, 0];

/** Index of the "no bound" stop, i.e. the far end of the scale. */
const OFF_INDEX = TIME_STOPS.length - 1;

/** Every stop that is an actual bound, ascending - the off stop excluded. */
const LIMITS = TIME_STOPS.slice(0, OFF_INDEX);

/**
 * Snaps an arbitrary bound to the stop the control can show.
 *
 * Rounds **up**: a bound rounded down would drop recipes the caller asked
 * to see, where rounding up only adds a few they did not ask for. Anything
 * past the last limit, and anything that is not a positive number, reads as
 * off - past the last limit there is no tighter stop left, and no bound is
 * the one that keeps every recipe the caller wanted.
 */
export function snapMaxMinutes(minutes: number): number {
	if (!Number.isFinite(minutes) || minutes <= 0) {
		return 0;
	}
	return LIMITS.find((limit) => limit >= minutes) ?? 0;
}

/** Position of a bound on the scale, snapping it first. */
export function timeStopIndex(minutes: number): number {
	const snapped = snapMaxMinutes(minutes);
	return snapped === 0 ? OFF_INDEX : LIMITS.indexOf(snapped);
}

/** The bound at a position on the scale; anything off the scale is off. */
export function timeStopMinutes(index: number): number {
	if (!Number.isInteger(index) || index < 0 || index > OFF_INDEX) {
		return 0;
	}
	return TIME_STOPS[index];
}
