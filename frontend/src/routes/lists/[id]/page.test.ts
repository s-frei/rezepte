import { describe, expect, it, vi } from 'vitest';
import { load } from './+page';

const listPlans = vi.fn();
const getShoppingList = vi.fn();
vi.mock('#lib/api/mealplan.js', () => ({ listPlans: () => listPlans() }));
vi.mock('#lib/api/shopping.js', () => ({ getShoppingList: (id: string) => getShoppingList(id) }));

type LoadEvent = Parameters<typeof load>[0];

describe('list route load', () => {
	it('asks for the plans while the list is still loading', async () => {
		let answer!: (list: unknown) => void;
		getShoppingList.mockReturnValue(new Promise((resolve) => (answer = resolve)));
		listPlans.mockResolvedValue({ plans: [{ id: 'home' }] });
		const loading = load({ params: { id: 'l1' } } as unknown as LoadEvent);
		expect(listPlans).toHaveBeenCalled();
		answer({ entries: [{ planId: 'home' }] });
		expect(await loading).toMatchObject({ plans: ['home'] });
	});
});
