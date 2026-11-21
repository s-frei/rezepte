import { getContext, setContext } from 'svelte';
import type { PlanEntry } from '#lib/api/mealplan.js';

const key = Symbol('plan-to-list');

/** The plan page opens "Auf eine Liste…" for one entry; its cards and menus ask for it. */
export const setToList = (open: (entry: PlanEntry) => void) => setContext(key, open);
export const getToList = () => getContext<((entry: PlanEntry) => void) | undefined>(key);
