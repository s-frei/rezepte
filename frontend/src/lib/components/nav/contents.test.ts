import { describe, expect, it } from 'vitest';
import { byGroup } from './contents';

describe('byGroup', () => {
	it('keeps ungrouped entries in one run', () => {
		const entries = [{ id: 'a' }, { id: 'b' }];
		expect(byGroup(entries)).toEqual([{ label: undefined, entries }]);
	});

	it('starts a run where the group changes, in order', () => {
		const runs = byGroup([
			{ id: 'profile', group: 'For you' },
			{ id: 'api', group: 'For you' },
			{ id: 'users', group: 'Household' }
		]);
		expect(runs.map((run) => [run.label, run.entries.map((entry) => entry.id)])).toEqual([
			['For you', ['profile', 'api']],
			['Household', ['users']]
		]);
	});
});
