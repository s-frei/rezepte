import { api } from './client';
import type { ListPerson } from './shopping';

/** A shopping list a plan entry is on, as far as the reader can access it. */
export type LinkedList = {
	id: string;
	name: string;
	openCount: number;
};

/** A recipe or a free text on a day, or on the tray (`day` null). */
export type PlanEntry = {
	id: string;
	/** `YYYY-MM-DD`, a date and never an instant; null on the tray. */
	day: string | null;
	position: number;
	/** Null for free text, and once the recipe is deleted; the title stays. */
	recipeId: string | null;
	recipeSlug: string | null;
	title: string;
	/** Null for free text. */
	servings: number | null;
	/** The recipe's cover image id. */
	photo: string | null;
	/** Null once the account of whoever added it is gone. */
	createdBy: ListPerson | null;
	lists: LinkedList[];
};

export type MealPlan = {
	id: string;
	/** Null for the personal plan; the client words it (`plan_personal`). */
	name: string | null;
	personal: boolean;
	/** Counts the plan's changes; the live stream sends it too. */
	revision: number;
	owner: ListPerson;
	/** Accepted members, without the owner. */
	members: ListPerson[];
	/** Invited and not yet answered; only the creator sees them. */
	pending: ListPerson[];
	/** The span the entries are for, both ends included. */
	from: string;
	to: string;
	/** The tray first, then by day and position. */
	entries: PlanEntry[];
	/** Recipe entries on no list the reader can access. */
	unlisted: number;
	/** The accessible lists the entries are on, most entries first. */
	lists: LinkedList[];
};

export type PlanSummary = {
	id: string;
	name: string | null;
	personal: boolean;
	owner: ListPerson;
	members: ListPerson[];
	updatedAt: string;
};

export type PlanInvitation = {
	planId: string;
	name: string | null;
	inviter: ListPerson;
};

export type Staple = { nameKey: string; name: string };

export type PreviewEntry = {
	entryId: string;
	title: string;
	servings: number;
	alreadyListed: boolean;
	ingredients: {
		group: number;
		index: number;
		name: string;
		quantity?: number;
		unit?: string;
		staple: boolean;
	}[];
};

/** The days a plan answer carries entries for; every change answers with the plan for its range. */
export type PlanRange = { from: string; to: string };

const base = (id: string) => `/meal-plans/${encodeURIComponent(id)}`;
const entry = (id: string, entryId: string) => `${base(id)}/entries/${encodeURIComponent(entryId)}`;
const span = ({ from, to }: PlanRange) => `?${new URLSearchParams({ from, to })}`;
const send = <T>(path: string, method: string, body?: unknown) =>
	api<T>(path, { method, body: body === undefined ? undefined : JSON.stringify(body) });

export const listPlans = () =>
	api<{ plans: PlanSummary[]; invitations: PlanInvitation[] }>('/meal-plans');

export const createPlan = (name: string, range: PlanRange) =>
	send<MealPlan>(`/meal-plans${span(range)}`, 'POST', { name });

export const getPlan = (id: string, range: PlanRange) => api<MealPlan>(base(id) + span(range));

export const renamePlan = (id: string, name: string, range: PlanRange) =>
	send<MealPlan>(base(id) + span(range), 'PATCH', { name });

export const deletePlan = (id: string) => send<void>(base(id), 'DELETE');

/** A recipe (`servings` left out: its own) or a free text, on `day` or on the tray (null). */
export const addPlanEntry = (
	id: string,
	input: { day: string | null; recipeId?: string; title?: string; servings?: number },
	range: PlanRange
) =>
	send<MealPlan>(`${base(id)}/entries${span(range)}`, 'POST', {
		...input,
		day: input.day ?? undefined
	});

/** To `day` (null: the tray) at `position`, from 0; left out, the day's end. */
export const movePlanEntry = (
	id: string,
	entryId: string,
	day: string | null,
	position: number | undefined,
	range: PlanRange
) => send<MealPlan>(entry(id, entryId) + span(range), 'PATCH', { day, position });

export const setPlanServings = (id: string, entryId: string, servings: number, range: PlanRange) =>
	send<MealPlan>(entry(id, entryId) + span(range), 'PATCH', { servings });

/** Restorable for a day with `restorePlanEntry`, list blocks and ticks included. */
export const removePlanEntry = (id: string, entryId: string, range: PlanRange) =>
	send<MealPlan>(entry(id, entryId) + span(range), 'DELETE');

export const restorePlanEntry = (id: string, entryId: string, range: PlanRange) =>
	send<MealPlan>(`${entry(id, entryId)}/restore${span(range)}`, 'POST');

/** Onto the list `listId`, or a new one named `newListName`. */
export const putPlanOnList = (
	id: string,
	input: {
		listId?: string;
		newListName?: string;
		picks: { entryId: string; ingredients: { group: number; index: number }[] }[];
	},
	range: PlanRange
) => send<{ plan: MealPlan; listId: string }>(`${base(id)}/shopping${span(range)}`, 'POST', input);

export const getShoppingPreview = (id: string, range: PlanRange, tray: boolean) =>
	api<{ entries: PreviewEntry[] }>(
		`${base(id)}/shopping-preview?${new URLSearchParams({ ...range, tray: String(tray) })}`
	);

export const getStaples = (id: string) => api<{ staples: Staple[] }>(`${base(id)}/staples`);

export const setStaple = (id: string, name: string, staple: boolean) =>
	send<{ staples: Staple[] }>(`${base(id)}/staples`, 'PUT', { name, staple });

export const invitePlan = (id: string, userId: string, range: PlanRange) =>
	send<MealPlan>(`${base(id)}/invitations${span(range)}`, 'POST', { userId });

/** Removes a member, or withdraws an invitation. */
export const removePlanMember = (id: string, userId: string, range: PlanRange) =>
	send<MealPlan>(`${base(id)}/members/${encodeURIComponent(userId)}${span(range)}`, 'DELETE');

export const acceptPlan = (id: string, range: PlanRange) =>
	send<MealPlan>(`${base(id)}/membership${span(range)}`, 'POST');

/** Leaves a plan, or declines its invitation. */
export const leavePlan = (id: string) => send<void>(`${base(id)}/membership`, 'DELETE');

/** The server-sent events stream of one plan: an event per change, see service/internal/mealplan. */
export const planEventsUrl = (id: string) => `/api/v1${base(id)}/events`;

/** A new secret calendar address for the reader; the old one stops working. */
export const createCalendarFeed = () => send<{ url: string }>('/calendar-feed', 'POST');

export const deleteCalendarFeed = () => send<void>('/calendar-feed', 'DELETE');
