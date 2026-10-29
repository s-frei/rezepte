import { getPasswordReset } from '#lib/api/auth.js';
import { getOidc } from '#lib/api/oidc.js';
import type { PageLoad } from './$types';

// The provider button and the "Forgot password" link are extras, so a
// failed lookup simply leaves them out. The page waits a moment for both,
// so it opens whole rather than growing under the fields a frame later; a
// slower answer still fills them in when it comes.
export const load: PageLoad = async () => {
	const oidc = getOidc().catch(() => null);
	const resetAvailable = getPasswordReset().then(
		(info) => info.available,
		() => false
	);
	await Promise.race([
		Promise.all([oidc, resetAvailable]),
		new Promise((resolve) => setTimeout(resolve, 300))
	]);
	return { oidc, resetAvailable };
};
