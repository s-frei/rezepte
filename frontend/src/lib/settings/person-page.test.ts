import { describe, expect, it } from 'vitest';
import type { PersonEntry, UserRole } from '#lib/api/users.js';
import { canOpenPerson, personPageTarget } from './person-page';

const person = (id: string, role: UserRole): PersonEntry => ({
	id,
	username: id,
	displayName: id,
	color: 'amber',
	role,
	avatarId: null
});

const owner = { id: 'owner', role: 'superadmin' as const };
const admin = { id: 'admin', role: 'admin' as const };

describe('personPageTarget', () => {
	it('shows a member to the owner with the profile editable', () => {
		const target = personPageTarget(owner, person('jonas', 'user'));
		expect(target.kind).toBe('show');
		expect(target.kind === 'show' && target.permissions.editProfile).toBe(true);
	});

	it('shows a member to a plain admin without the profile editable', () => {
		const target = personPageTarget(admin, person('jonas', 'user'));
		expect(target.kind).toBe('show');
		expect(target.kind === 'show' && target.permissions.editProfile).toBe(false);
	});

	it('sends a plain admin from another admin back to the list', () => {
		expect(personPageTarget(admin, person('mia', 'admin'))).toEqual({
			kind: 'redirect',
			to: '/settings/users'
		});
	});

	it('sends anyone from the owner back to the list', () => {
		expect(personPageTarget(admin, person('owner', 'superadmin'))).toEqual({
			kind: 'redirect',
			to: '/settings/users'
		});
		expect(personPageTarget({ id: 'u', role: 'user' }, person('owner', 'superadmin'))).toEqual({
			kind: 'redirect',
			to: '/settings/users'
		});
	});

	it('sends the viewer from their own entry to the profile page', () => {
		expect(personPageTarget(owner, person('owner', 'superadmin'))).toEqual({
			kind: 'redirect',
			to: '/settings'
		});
		expect(personPageTarget(admin, person('admin', 'admin'))).toEqual({
			kind: 'redirect',
			to: '/settings'
		});
	});

	it('says an unknown person is missing', () => {
		expect(personPageTarget(owner, undefined)).toEqual({ kind: 'missing' });
	});
});

describe('canOpenPerson', () => {
	const none = {
		changeRole: false,
		manageAccount: false,
		editProfile: false,
		toggleSharing: false
	};

	it('is false without any permission', () => {
		expect(canOpenPerson(none)).toBe(false);
	});

	it('is true with any one permission', () => {
		for (const key of Object.keys(none) as (keyof typeof none)[]) {
			expect(canOpenPerson({ ...none, [key]: true })).toBe(true);
		}
	});
});
