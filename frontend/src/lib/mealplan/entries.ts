import { resolve } from '$app/paths';
import type { PlanEntry } from '#lib/api/mealplan.js';

/** The tray's key among the days: an entry with no day. */
export const TRAY = 'tray';

/** The entries of each day of `days` and of the tray, in their order. */
export function byZone(entries: PlanEntry[], days: string[]): Record<string, PlanEntry[]> {
	const zones: Record<string, PlanEntry[]> = { [TRAY]: [] };
	for (const day of days) zones[day] = [];
	for (const entry of [...entries].sort((a, b) => a.position - b.position))
		zones[entry.day ?? TRAY]?.push(entry);
	return zones;
}

/**
 * `entries` with `id` moved to `day` (null: the tray) at `index` of that
 * day, the way the server will answer: positions count from 0 within each
 * day. A day outside `entries`' range simply takes the entry out of sight.
 */
export function placeEntry(
	entries: PlanEntry[],
	id: string,
	day: string | null,
	index: number
): PlanEntry[] {
	const moving = entries.find((e) => e.id === id);
	if (!moving) return entries;
	const rest = entries.filter((e) => e.id !== id);
	const target = rest.filter((e) => e.day === day).sort((a, b) => a.position - b.position);
	target.splice(Math.max(0, Math.min(index, target.length)), 0, { ...moving, day });
	const placed = new Map(target.map((e, position) => [e.id, { ...e, position }]));
	const from = rest
		.filter((e) => e.day === moving.day && moving.day !== day)
		.sort((a, b) => a.position - b.position)
		.map((e, position) => ({ ...e, position }));
	const renumbered = new Map(from.map((e) => [e.id, e]));
	return [...rest.map((e) => placed.get(e.id) ?? renumbered.get(e.id) ?? e), placed.get(id)!];
}

/** A recipe entry's page at the planned servings; null for a note or a deleted recipe. */
export function recipeHref(entry: PlanEntry) {
	if (!entry.recipeSlug) return null;
	const page = resolve('/recipes/[slug]', { slug: entry.recipeSlug });
	return entry.servings === null ? page : (`${page}?servings=${entry.servings}` as typeof page);
}

/** The longest note, in runes, as the server takes it. */
export const MAX_NOTE = 120;

/**
 * The add sheet's rows: the recipes found for exactly the text in the field
 * (an answer for an earlier text, still on its way out, is left out, so
 * Enter never takes a recipe nobody asked for), then the text as a note.
 */
export function addRows<R extends { id: string }>(
	query: string,
	found: { query: string; items: R[] } | null
): { id: string; recipe?: R; note?: string }[] {
	const text = query.trim();
	const recipes = found && found.query === text ? found.items : [];
	const note = [...text].slice(0, MAX_NOTE).join('');
	return [
		...recipes.map((recipe) => ({ id: `plan-add-${recipe.id}`, recipe })),
		...(note ? [{ id: 'plan-add-note', note }] : [])
	];
}
