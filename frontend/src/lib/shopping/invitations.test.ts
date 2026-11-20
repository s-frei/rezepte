import { expect, it } from 'vitest';
import { invitations } from './invitations.svelte';

it('counts list and plan invitations, one in the singular', () => {
	invitations.lists = 1;
	invitations.plans = 0;
	expect(invitations.label).toBe('1 invitation');
	invitations.plans = 2;
	expect(invitations.count).toBe(3);
	expect(invitations.label).toBe('3 invitations');
});
