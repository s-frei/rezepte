import type { IngredientGroup, ImageVariant, Step } from './recipes';

// Contract types (binding for both the backend and the frontend; see
// service/internal/share/public.go, PublicRecipe). This is its own narrow
// type, not RecipeContent: a public link answers with exactly these keys
// and nothing a household member's session would carry.

export type PublicImage = { id: string; width: number; height: number };

export type PublicRecipe = {
	title: string;
	description: string;
	servings: number;
	prepMinutes: number | null;
	cookMinutes: number | null;
	sourceUrl: string | null;
	sourceName: string | null;
	tags: string[];
	ingredientGroups: IngredientGroup[];
	steps: Step[];
	images: PublicImage[];
	coverImageId: string | null;
	/** Whether the page names Rezepte at its foot - the owner's setting, since a stranger has no session to read it with. */
	attribution: boolean;
};

/**
 * Reads the recipe behind a public link. Deliberately bypasses `api()`, like
 * `fetchVersion`: a stranger opening `/s/<token>` has no session, so a 404
 * (unknown, revoked, paused, expired - the backend collapses them all into
 * the same answer) must never trigger the app's login redirect. Resolves to
 * null for any non-2xx response *and* for any error along the way - a
 * dropped connection, an offline stranger, a malformed body - so the page
 * always gets to render its own "not available" state and never falls
 * through to SvelteKit's generic error page, which would drop this route's
 * header along with it.
 */
export async function getPublicRecipe(token: string): Promise<PublicRecipe | null> {
	try {
		const res = await fetch(`/api/v1/public/shares/${encodeURIComponent(token)}`, {
			headers: { Accept: 'application/json' }
		});
		if (!res.ok) {
			return null;
		}
		return (await res.json()) as PublicRecipe;
	} catch {
		return null;
	}
}

/**
 * URL of one rendition of a public share's photo, served without a session
 * at /public-images/{token}/{imageId}/{variant}.jpg. Passed as `RecipeView`'s
 * `imageSrc` on the public page.
 */
export function publicImageUrl(token: string, imageId: string, variant: ImageVariant): string {
	return `/public-images/${encodeURIComponent(token)}/${encodeURIComponent(imageId)}/${variant}.jpg`;
}
