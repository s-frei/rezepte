import type { PageLoad } from './$types';

// Waits for the root layout, so a signed-out visit goes to the login with
// this page as `next`. The page decides itself whether the viewer may see
// the person: it needs the people list for that, and loads it with the rest.
export const load: PageLoad = async ({ params, parent }) => {
	const { user } = await parent();
	return { username: params.username, viewer: user };
};
