import { createGetUrl } from 'fumadocs-core/source';

// Trailing slash required: a relative metadata URL (e.g. a future `./og.png`)
// resolves against this with `new URL()`, and without it the sub-path is
// dropped - `new URL('./og.png', 'https://host/rezepte')` resolves to
// `https://host/og.png`, not `https://host/rezepte/og.png`.
export const SITE_URL = 'https://s-frei.github.io/rezepte/';

// Next merges metadata shallowly: a page-level `openGraph` REPLACES the root
// layout's rather than extending it. Every page that sets its own og:title has
// to spread these in, or it silently loses og:type, og:site_name and og:locale.
export const OG_SHARED = { type: 'website', siteName: 'Rezepte', locale: 'en_US' } as const;

const getContentUrl = createGetUrl('/llms.mdx');

export function getPageMarkdownUrl(page: { slugs: string[] }) {
	return getContentUrl([...page.slugs, 'content.md']);
}
