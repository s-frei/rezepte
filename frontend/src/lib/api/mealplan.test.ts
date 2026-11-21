import { getLocalTimeZone, today } from '@internationalized/date';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { addPlanEntry, getPlan, movePlanEntry } from './mealplan';

function mockFetch() {
	const fn = vi.fn(
		async () => new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } })
	);
	vi.stubGlobal('fetch', fn);
	return fn;
}

const call = (fn: ReturnType<typeof mockFetch>) =>
	fn.mock.calls[0] as unknown as [string, RequestInit];

const week = { from: '2026-11-16', to: '2026-11-22' };
// The reader's today goes along: the server leaves the days before it out of `unlisted`.
const now = () => today(getLocalTimeZone()).toString();

afterEach(() => vi.unstubAllGlobals());

describe('meal plan api', () => {
	it('asks for a plan by its range', async () => {
		const fn = mockFetch();
		await getPlan('p', week);
		expect(call(fn)[0]).toBe(`/api/v1/meal-plans/p?from=2026-11-16&to=2026-11-22&today=${now()}`);
	});

	// The server tells a left-out day (keep it) from null (the tray).
	it('moves an entry to the tray with an explicit null day', async () => {
		const fn = mockFetch();
		await movePlanEntry('p', 'e', null, 0, week);
		const [url, init] = call(fn);
		expect(url).toBe(`/api/v1/meal-plans/p/entries/e?from=2026-11-16&to=2026-11-22&today=${now()}`);
		expect(init.method).toBe('PATCH');
		expect(JSON.parse(init.body as string)).toEqual({ day: null, position: 0 });
	});

	it('adds to the tray by leaving the day out', async () => {
		const fn = mockFetch();
		await addPlanEntry('p', { day: null, title: 'Reste' }, week);
		expect(JSON.parse(call(fn)[1].body as string)).toEqual({ title: 'Reste' });
	});
});
