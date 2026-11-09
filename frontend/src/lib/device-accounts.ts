import { listDeviceAccounts, type DeviceAccount } from '#lib/api/auth.js';

/**
 * The accounts this device knows, or none when they cannot be asked: sign-out
 * and the 401 path must never fail on the list, they fall back to /login.
 */
export async function knownAccounts(): Promise<DeviceAccount[]> {
	try {
		return await listDeviceAccounts();
	} catch {
		return [];
	}
}

/**
 * Where every signed-out moment goes - a sign-out, the root load's 401, a
 * 401 mid-use: "Who's cooking?" while the device knows someone, else the
 * login page. `next`, a path, comes along for after the sign-in or switch.
 */
export async function signedOutTarget(next?: string): Promise<string> {
	const query = next === undefined ? '' : `?next=${encodeURIComponent(next)}`;
	return (await knownAccounts()).length > 0 ? `/switch${query}` : `/login${query}`;
}
