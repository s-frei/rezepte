import { describe, expect, it } from 'vitest';
import type { UserAccount, UserRole } from '$lib/api/users';
import { compareUsernames, nextSort, sortUsers } from './user-sort';

function user(username: string, role: UserRole = 'user', displayName = username): UserAccount {
	return {
		id: username,
		username,
		displayName,
		role,
		color: 'amber',
		locale: 'en',
		createdAt: '2026-01-01T00:00:00Z'
	};
}

function names(users: UserAccount[]): string[] {
	return users.map((u) => u.username);
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

describe('sortUsers', () => {
	const users = [user('Zoe'), user('Änna'), user('bert', 'admin'), user('admin', 'superadmin')];

	it('sorts by name ascending and leaves the input alone', () => {
		const sorted = sortUsers(users, { column: 'name', direction: 'asc' });
		expect(names(sorted)).toEqual(['admin', 'Änna', 'bert', 'Zoe']);
		expect(names(users)).toEqual(['Zoe', 'Änna', 'bert', 'admin']);
	});

	it('reverses the name order', () => {
		const sorted = sortUsers(users, { column: 'name', direction: 'desc' });
		expect(names(sorted)).toEqual(['Zoe', 'bert', 'Änna', 'admin']);
	});

	// Same account twice under two spellings the collator cannot separate:
	// the order must not depend on which one the API happened to send first.
	it('orders a collator tie the same whichever way it arrives', () => {
		const pair = [user('user2'), user('user02')];
		const one = sortUsers(pair, { column: 'name', direction: 'asc' });
		const other = sortUsers([...pair].reverse(), { column: 'name', direction: 'asc' });
		expect(names(one)).toEqual(names(other));
	});

	it('sorts by rank, owner first, and breaks ties by name', () => {
		const sorted = sortUsers(users, { column: 'role', direction: 'asc' });
		expect(names(sorted)).toEqual(['admin', 'bert', 'Änna', 'Zoe']);
	});

	it('reverses rank and names together', () => {
		const sorted = sortUsers(users, { column: 'role', direction: 'desc' });
		expect(names(sorted)).toEqual(['Zoe', 'Änna', 'bert', 'admin']);
	});

	// The row's primary line is the display name, not the login name - a
	// household whose display names differ from their usernames must sort on
	// what the row shows, not on what it hides.
	it('sorts by display name rather than username', () => {
		const mismatched = [
			user('sam', 'user', 'Zora'),
			user('kim', 'user', 'Anna'),
			user('joe', 'user', 'Mira')
		];
		const sorted = sortUsers(mismatched, { column: 'name', direction: 'asc' });
		expect(sorted.map((u) => u.displayName)).toEqual(['Anna', 'Mira', 'Zora']);
	});

	// Display names are deliberately not unique, so two people sharing one
	// must still land in a stable, total order - the username breaks the tie.
	it('breaks a display-name tie with the username', () => {
		const sharedName = [user('zeb', 'user', 'Mia'), user('abe', 'user', 'Mia')];
		const sorted = sortUsers(sharedName, { column: 'name', direction: 'asc' });
		expect(names(sorted)).toEqual(['abe', 'zeb']);
	});
});

describe('nextSort', () => {
	it('starts a new column ascending', () => {
		expect(nextSort({ column: 'name', direction: 'desc' }, 'role')).toEqual({
			column: 'role',
			direction: 'asc'
		});
	});

	it('reverses the active column', () => {
		expect(nextSort({ column: 'name', direction: 'asc' }, 'name')).toEqual({
			column: 'name',
			direction: 'desc'
		});
	});
});
