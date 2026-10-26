import ArrowLeftRight from '@lucide/svelte/icons/arrow-left-right';
import Code from '@lucide/svelte/icons/code';
import Globe from '@lucide/svelte/icons/globe';
import KeyRound from '@lucide/svelte/icons/key-round';
import Mail from '@lucide/svelte/icons/mail';
import UserRound from '@lucide/svelte/icons/user-round';
import UsersRound from '@lucide/svelte/icons/users-round';
import { resolve } from '$app/paths';
import type { ResolvedPathname } from '$app/types';
import type { IconComponent } from '$lib/components/nav/contents';
import type { UserRole } from '$lib/api/users';
import { m } from '$lib/paraglide/messages';
import { isAdminRole } from '$lib/roles';

export type SettingsPageId =
	'profile' | 'shares' | 'api' | 'users' | 'mail' | 'sign-in' | 'transfer';

export type SettingsPage = {
	id: SettingsPageId;
	href: ResolvedPathname;
	label: string;
	icon: IconComponent;
	group: string;
};

/**
 * The pages of the settings area for a role, in the order and groups every
 * nav lists them: the settings sidebar, its contents sheet and the You sheet.
 * "For you" is the account's own, with API last as the page a kitchen needs
 * least; "Household" is what concerns everyone on the instance: the people
 * list, a list to read for everyone and where admins manage accounts, then
 * email and sign-in (the owner's alone) and import and export (admins).
 */
export function settingsPages(role: UserRole | undefined): SettingsPage[] {
	const personal = m.settings_group_personal();
	const household = m.settings_group_household();
	const pages: SettingsPage[] = [
		{
			id: 'profile',
			href: resolve('/settings'),
			label: m.settings_nav_profile(),
			icon: UserRound,
			group: personal
		},
		{
			id: 'shares',
			href: resolve('/settings/shares'),
			label: m.settings_nav_shares(),
			icon: Globe,
			group: personal
		},
		{
			id: 'api',
			href: resolve('/settings/api'),
			label: m.settings_nav_api(),
			icon: Code,
			group: personal
		},
		{
			id: 'users',
			href: resolve('/settings/users'),
			label: m.settings_nav_users(),
			icon: UsersRound,
			group: household
		},
		{
			id: 'mail',
			href: resolve('/settings/mail'),
			label: m.settings_nav_mail(),
			icon: Mail,
			group: household
		},
		{
			id: 'sign-in',
			href: resolve('/settings/sign-in'),
			label: m.settings_nav_sign_in(),
			icon: KeyRound,
			group: household
		},
		{
			id: 'transfer',
			href: resolve('/settings/transfer'),
			label: m.settings_nav_transfer(),
			icon: ArrowLeftRight,
			group: household
		}
	];
	return pages.filter(
		(page) =>
			((page.id !== 'mail' && page.id !== 'sign-in') || role === 'superadmin') &&
			(page.id !== 'transfer' || isAdminRole(role))
	);
}
