import { describe, expect, it } from 'vitest';
import type { PersonEntry, UserRole } from '$lib/api/users';
import { compareUsernames, groupPeople } from './people-groups';

function person(username: string, role: UserRole = 'user', displayName = username): PersonEntry {
	return { id: username, username, displayName, role, color: 'amber' };
}

/** Each group as `role: username, username`, the shape a reader scans. */
function shape(people: PersonEntry[]): string[] {
	return groupPeople(people).map((g) => `${g.role}: ${g.people.map((p) => p.username).join(', ')}`);
}

describe('compareUsernames', () => {
	// The bug this comparator exists for: SQLite's COLLATE NOCASE folds ASCII
	// only and files "Änna" after "Zoe".
	it('files an umlaut with its base letter', () => {
		expect(compareUsernames('Änna', 'Zoe')).toBeLessThan(0);
		expect(compareUsernames('Änna', 'Amsel')).toBeGreaterThan(0);
	});

	it('ignores case', () => {
		expect(compareUsernames('anna', 'Bert')).toBeLessThan(0);
		expect(compareUsernames('Bert', 'anna')).toBeGreaterThan(0);
	});

	it('orders digits by value, not by character', () => {
		expect(compareUsernames('user2', 'user10')).toBeLessThan(0);
	});

	// A tie would leave the order to however the two rows happened to arrive,
	// which is the whole bug. `numeric: true` calls "user02" and "user2" equal
	// on its own, and a weaker collator strength does the same to an accent.
	it('never calls two different names equal', () => {
		expect(compareUsernames('user02', 'user2')).not.toBe(0);
		expect(compareUsernames('user2', 'user02')).not.toBe(0);
		expect(compareUsernames('Anna', 'Änna')).not.toBe(0);
	});

	it('is antisymmetric on the pairs the collator ties', () => {
		expect(Math.sign(compareUsernames('user02', 'user2'))).toBe(
			-Math.sign(compareUsernames('user2', 'user02'))
		);
	});
});

describe('groupPeople', () => {
	it('puts the owner first, then admins, then members', () => {
		const people = [person('zoe'), person('bert', 'admin'), person('admin', 'superadmin')];
		expect(shape(people)).toEqual(['superadmin: admin', 'admin: bert', 'user: zoe']);
	});

	it('leaves out a role nobody holds', () => {
		expect(shape([person('admin', 'superadmin'), person('zoe')])).toEqual([
			'superadmin: admin',
			'user: zoe'
		]);
	});

	it('orders a group by name the way the reader reads it, and leaves the input alone', () => {
		const people = [person('Zoe'), person('Änna'), person('bert')];
		expect(shape(people)).toEqual(['user: Änna, bert, Zoe']);
		expect(people.map((p) => p.username)).toEqual(['Zoe', 'Änna', 'bert']);
	});

	// The row's primary line is the display name, not the login name - a
	// household whose display names differ from their usernames must be
	// ordered by what the row shows, not by what it hides.
	it('orders by display name rather than username', () => {
		const people = [
			person('sam', 'user', 'Zora'),
			person('kim', 'user', 'Anna'),
			person('joe', 'user', 'Mira')
		];
		expect(groupPeople(people)[0].people.map((p) => p.displayName)).toEqual([
			'Anna',
			'Mira',
			'Zora'
		]);
	});

	// Display names are deliberately not unique, so two people sharing one
	// must still land in a stable, total order - the username breaks the tie,
	// whichever way the two arrive.
	it('breaks a display-name tie with the username', () => {
		const a = person('abe', 'user', 'Mia');
		const z = person('zeb', 'user', 'Mia');
		expect(shape([z, a])).toEqual(['user: abe, zeb']);
		expect(shape([a, z])).toEqual(['user: abe, zeb']);
	});

	it('returns no groups for nobody', () => {
		expect(groupPeople([])).toEqual([]);
	});
});
