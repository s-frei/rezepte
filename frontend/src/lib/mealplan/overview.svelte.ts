import type { WeekEntry, WeekPlan } from '#lib/api/mealplan.js';
import { weekDays } from './week';

/**
 * The plans overview's order: "My plan" first, then the plans with meals
 * this week (most first), then the rest by their last change, so the plans
 * in use stay on top however many idle ones there are.
 */
export function byUse(a: WeekPlan, b: WeekPlan): number {
	return (
		Number(b.personal) - Number(a.personal) ||
		b.entries.length - a.entries.length ||
		b.updatedAt.localeCompare(a.updatedAt)
	);
}

/**
 * A named plan with nothing this week, on a day or on the tray: a chip in
 * the "empty this week" band, not a card. "My plan" is always a card.
 */
export const isIdle = (plan: WeekPlan): boolean =>
	!plan.personal && plan.entries.length === 0 && plan.tray === 0;

/** What a strip cell shows: a photo, a recipe's letter (also a deleted recipe's), a note's pencil. */
export type CellKind = 'photo' | 'letter' | 'note';

export const cellKind = (entry: WeekEntry): CellKind =>
	entry.kind === 'text' ? 'note' : entry.recipeId && entry.photo ? 'photo' : 'letter';

export type StripDay = {
	day: string;
	/** The day's meals in plan order; the cell shows the first, "+N" the rest. */
	meals: WeekEntry[];
	when: 'past' | 'today' | 'future';
};

/** The week from `monday` as seven cells, each knowing whether it is gone by. */
export function weekStrip(entries: WeekEntry[], monday: string, today: string): StripDay[] {
	return weekDays(monday).map((day) => ({
		day,
		meals: entries.filter((e) => e.day === day),
		when: day < today ? 'past' : day === today ? 'today' : 'future'
	}));
}

/**
 * Set by a plan page that goes back to the overview because its plan is
 * gone (deleted, left): the overview then puts the focus on its heading.
 */
export const arrival = $state({ overview: false });
