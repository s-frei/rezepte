import { expect, it } from 'vitest';
import { invitations } from './invitations.svelte';

it('names one invitation in the singular', () => {
	invitations.count = 1;
	expect(invitations.label).toBe('1 invitation');
	invitations.count = 3;
	expect(invitations.label).toBe('3 invitations');
});
