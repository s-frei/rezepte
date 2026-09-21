import { describe, expect, it } from 'vitest';
import { authorLabel, hasBeenEdited } from './authorship';

const untouched = {
	createdBy: 'u1',
	createdAt: '2026-09-20T20:10:26Z',
	updatedBy: 'u1',
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
		expect(hasBeenEdited({ ...untouched, updatedBy: 'u2' })).toBe(true);
	});
});

// The card shows initials, so the readable names live in the label a screen
// reader announces and a desktop hover reveals.
describe('authorLabel', () => {
	it('names the author alone when nobody else touched the recipe', () => {
		expect(authorLabel('admin', 'amber', 'admin', 'amber')).toBe('Angelegt von admin');
	});

	it('names both when somebody else edited last', () => {
		expect(authorLabel('admin', 'amber', 'mara', 'teal')).toBe(
			'Angelegt von admin, zuletzt bearbeitet von mara'
		);
	});

	// Display names are deliberately not unique - two members may both be
	// "Mia" - so the same name with a different colour is still two people.
	it('names both when two people share a name but not a colour', () => {
		expect(authorLabel('Mia', 'amber', 'Mia', 'teal')).toBe(
			'Angelegt von Mia, zuletzt bearbeitet von Mia'
		);
	});
});
