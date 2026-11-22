import type { PantryItem } from '#lib/api/recipes.js';
import { covers } from './covers.js';

/** The user's own tick of this item: one with its key. */
export function ownTick(item: PantryItem, ticked: PantryItem[]): PantryItem | null {
	return ticked.find((t) => t.key === item.key) ?? null;
}

/**
 * The tick that covers an item the user did not tick themselves ("Essig"
 * covers "Rotweinessig"), or null; an own tick is never covered by itself.
 */
export function coveredBy(item: PantryItem, ticked: PantryItem[]): PantryItem | null {
	if (ownTick(item, ticked)) return null;
	return ticked.find((t) => covers(t.key, item.key)) ?? null;
}

/** The header's count: the user's own ticks, once per key; covered pills never count. */
export function ownTickCount(ticked: PantryItem[]): number {
	return new Set(ticked.map((t) => t.key)).size;
}

/** The row plus every tick that covers nothing in it, so each tick has a pill to untick. */
export function withTicks(row: PantryItem[], ticked: PantryItem[]): PantryItem[] {
	return [...row, ...ticked.filter((t) => !row.some((n) => covers(t.key, n.key)))];
}

/** The pills a capped row hides: past `cap`, all but the ticked and covered ones. */
export function beyondCap(items: PantryItem[], ticked: PantryItem[], cap: number): PantryItem[] {
	return items.filter((item, i) => i >= cap && !ownTick(item, ticked) && !coveredBy(item, ticked));
}
