import { describe, expect, it } from 'vitest';
import type { Person } from '$lib/api/recipes';
import { authorLabel, hasBeenEdited } from './authorship';

const admin: Person = { id: 'u1', username: 'admin', displayName: 'admin', color: 'amber' };
const mara: Person = { id: 'u2', username: 'mara', displayName: 'mara', color: 'teal' };

const untouched = {
	createdBy: admin,
	createdAt: '2026-09-20T20:10:26Z',
	updatedBy: admin,
	updatedAt: '2026-09-20T20:10:26Z'
};

describe('hasBeenEdited', () => {
	it('is false for a recipe nobody has edited', () => {
		expect(hasBeenEdited(untouched)).toBe(false);
	});

	it('is true once the timestamp moves', () => {
		expect(hasBeenEdited({ ...untouched, updatedAt: '2026-09-21T08:00:00Z' })).toBe(true);
	});

	// Timestamps have second resolution, so an edit that lands in the same
	// second as the write before it leaves updated_at untouched. Without the
	// editor comparison, somebody else's edit would go uncredited.
	it('is true when somebody else edited within the same second', () => {
		expect(hasBeenEdited({ ...untouched, updatedBy: mara })).toBe(true);
	});
});

// The card shows initials, so the readable names live in the label a screen
// reader announces and a desktop hover reveals.
describe('authorLabel', () => {
	it('names the author alone when nobody else touched the recipe', () => {
		expect(authorLabel(admin, admin)).toBe('Added by admin');
	});

	it('names both when somebody else edited last', () => {
		expect(authorLabel(admin, mara)).toBe('Added by admin, last edited by mara');
	});

	// Display names are deliberately not unique - two members may both be
	// "Mia" - so two people are told apart by id, whatever they are called.
	it('names both when two people share a display name', () => {
		const mia = { ...admin, id: 'u3', username: 'mia', displayName: 'Mia' };
		const otherMia = { ...mara, id: 'u4', username: 'mia2', displayName: 'Mia' };
		expect(authorLabel(mia, otherMia)).toBe('Added by Mia, last edited by Mia');
	});
});
