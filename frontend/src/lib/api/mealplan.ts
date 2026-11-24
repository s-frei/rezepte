import { getLocalTimeZone, today } from '@internationalized/date';
import type { ImageFocus } from '#lib/images/focus.js';
import { api } from './client';
import type { ListPerson } from './shopping';

/** A shopping list a plan entry is on, as far as the reader can access it. */
export type LinkedList = {
	id: string;
	name: string;
	openCount: number;
	/** The plan's entries on the list; only on `MealPlan.lists` and `WeekPlan.lists`. */
	entries?: number;
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
	/** The cover's pixel size and focus point, null with `photo`. */
	photoWidth: number | null;
	photoHeight: number | null;
	photoFocus: ImageFocus | null;
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
	/** Whether the plan holds an entry anywhere, outside `from`–`to` too. */
	hasEntries: boolean;
	/** Recipe entries on no list the reader can access, on the tray or on the reader's today or later (`span` sends it). */
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

/** A plan entry on the plans page: on a day of the week asked for. */
export type WeekEntry = {
	id: string;
	/** `YYYY-MM-DD`. */
	day: string;
	/** A recipe whose recipe is deleted stays a recipe, with `recipeId` and `photo` null. */
	kind: 'recipe' | 'text';
	recipeId: string | null;
	title: string;
	/** The recipe's cover image id. */
	photo: string | null;
	/** The cover's pixel size and focus point, null with `photo`. */
	photoWidth: number | null;
	photoHeight: number | null;
	photoFocus: ImageFocus | null;
};

/** A plan with one week of it, as the plans page shows it. */
export type WeekPlan = PlanSummary & {
	/** The week's entries on a day, by day and position; the tray is left out. */
	entries: WeekEntry[];
	/** Recipe entries of the week from today on and the tray on no list the reader can access, as `MealPlan.unlisted`. */
	unlisted: number;
	/** As `MealPlan.lists` for the same week, the tray included. */
	lists: LinkedList[];
	/** Entries on the tray. */
	tray: number;
	/** Entries on a day after the range. */
	later: number;
	/** The first of them, by day and position; absent when none. */
	next?: WeekEntry;
	/** Invitees still waiting; empty unless the reader made the plan. */
	pending: ListPerson[];
};

export type PlanInvitation = {
	planId: string;
	name: string | null;
	inviter: ListPerson;
};

export type Staple = { nameKey: string; name: string };

export type PreviewEntry = {
	entryId: string;
	/** `YYYY-MM-DD`; null on the tray. */
	day: string | null;
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
/** The range and the reader's today, which decides the past days `unlisted` leaves out. */
const span = ({ from, to }: PlanRange) =>
	`?${new URLSearchParams({ from, to, today: today(getLocalTimeZone()).toString() })}`;
const send = <T>(path: string, method: string, body?: unknown) =>
	api<T>(path, { method, body: body === undefined ? undefined : JSON.stringify(body) });

export const listPlans = () =>
	api<{ plans: PlanSummary[]; invitations: PlanInvitation[] }>('/meal-plans');

/**
 * Every plan of the reader's with `range` of it, and the invitations. Pass the
 * reader's own week (`today(getLocalTimeZone())`): left out, the server
 * picks the current week in its own time zone. `span` adds the reader's today.
 */
/** `redirect: false` for the overview's load, whose page redirects on a 401 itself (`api`). */
export const getPlansWeek = (range: PlanRange, options?: { redirect?: boolean }) =>
	api<PlanRange & { plans: WeekPlan[]; invitations: PlanInvitation[] }>(
		`/meal-plans/week${span(range)}`,
		{},
		options
	);

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
		/** Skip an entry only when it is on this list already (one entry from its ⋯); left out, one on any list is skipped. */
		alsoListed?: boolean;
	},
	range: PlanRange
) => send<{ plan: MealPlan; listId: string }>(`${base(id)}/shopping${span(range)}`, 'POST', input);

export const getShoppingPreview = (id: string, range: PlanRange, tray: boolean) =>
	api<{ entries: PreviewEntry[] }>(
		`${base(id)}/shopping-preview?${new URLSearchParams({ ...range, tray: String(tray) })}`
	);

/** The preview of one recipe entry, on its day or the tray, for the one-entry dialog. */
export const getEntryShoppingPreview = (id: string, entryId: string) =>
	api<{ entries: PreviewEntry[] }>(
		`${base(id)}/shopping-preview?${new URLSearchParams({ entry: entryId })}`
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

/** Whether the reader's calendar address works, and since when; the address is shown only by `createCalendarFeed`. */
export const getCalendarFeed = () =>
	api<{ active: boolean; createdAt: string | null }>('/calendar-feed');
