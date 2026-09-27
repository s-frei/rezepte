import { describe, expect, it } from 'vitest';
import type { UserRole } from '$lib/api/users';
import { rowPermissions } from './row-permissions';

const none = { changeRole: false, manageAccount: false, editProfile: false, toggleSharing: false };
const roles: UserRole[] = ['user', 'admin', 'superadmin'];

describe('rowPermissions', () => {
	// The page is open to every account, so the viewer's rank is the first
	// gate: a member reads the list and acts on no row, their own included.
	it('grants a member nothing on any row', () => {
		for (const target of roles) {
			for (const isSelf of [false, true]) {
				expect(rowPermissions('user', { role: target }, isSelf)).toEqual(none);
			}
		}
	});

	it('grants nobody anything on the owner row', () => {
		for (const actor of roles) {
			expect(rowPermissions(actor, { role: 'superadmin' }, actor === 'superadmin')).toEqual(none);
			expect(rowPermissions(actor, { role: 'superadmin' }, false)).toEqual(none);
		}
	});

	it('lets the owner do everything to an admin or a member', () => {
		const all = { changeRole: true, manageAccount: true, editProfile: true, toggleSharing: true };
		expect(rowPermissions('superadmin', { role: 'admin' }, false)).toEqual(all);
		expect(rowPermissions('superadmin', { role: 'user' }, false)).toEqual(all);
	});

	it('lets an admin reset, delete and switch sharing for a member, nothing more', () => {
		expect(rowPermissions('admin', { role: 'user' }, false)).toEqual({
			...none,
			manageAccount: true,
			toggleSharing: true
		});
	});

	it('leaves another admin to the owner', () => {
		expect(rowPermissions('admin', { role: 'admin' }, false)).toEqual(none);
	});

	it('leaves an admin their own row to the profile page', () => {
		expect(rowPermissions('admin', { role: 'admin' }, true)).toEqual(none);
	});
});
