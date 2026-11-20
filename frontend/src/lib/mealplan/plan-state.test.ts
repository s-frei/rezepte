import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getPlan, type MealPlan } from '#lib/api/mealplan.js';
import { PlanState } from './plan-state.svelte';

vi.mock('#lib/api/mealplan.js', () => ({ getPlan: vi.fn() }));
const get = vi.mocked(getPlan);

const make = (from: string, revision = 0, title = 'A') =>
	({
		id: 'p1',
		from,
		to: from,
		revision,
		entries: [{ id: 'e1', title }]
	}) as unknown as MealPlan;

describe('PlanState', () => {
	beforeEach(() => vi.resetAllMocks());

	it('drops an answer for the week the page has left', async () => {
		const state = new PlanState(make('2026-11-16'), vi.fn());
		let resolve!: (plan: MealPlan) => void;
		const done = state.apply(new Promise((r) => (resolve = r)));
		state.load(make('2026-11-23'));
		resolve(make('2026-11-16', 5));
		await done;
		expect(state.plan.from).toBe('2026-11-23');
	});

	it('refetches the shown week on a newer revision only', async () => {
		get.mockResolvedValue(make('2026-11-16', 3, 'B'));
		const state = new PlanState(make('2026-11-16', 2), vi.fn());
		await state.follow(2);
		expect(get).not.toHaveBeenCalled();
		await state.follow(3);
		expect(get).toHaveBeenCalledWith('p1', { from: '2026-11-16', to: '2026-11-16' });
		expect(state.plan.entries[0].title).toBe('B');
	});
});
