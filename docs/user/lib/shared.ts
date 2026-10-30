import { createGetUrl } from 'fumadocs-core/source';

// Trailing slash required: a relative metadata URL (e.g. a future `./og.png`)
// resolves against this with `new URL()`, and without it the sub-path is
// dropped - `new URL('./og.png', 'https://host/rezepte')` resolves to
// `https://host/og.png`, not `https://host/rezepte/og.png`.
export const SITE_URL = 'https://s-frei.github.io/rezepte/';

// The published address of a page URL ("/" or "/guide/cook-mode"), slashed
// like the export (`trailingSlash: true`): the one form search engines and
// link previews should count, since the others redirect to it.
export function publishedUrl(url: string) {
	return url === '/' ? SITE_URL : `${SITE_URL}${url.slice(1)}/`;
}

// What a search result shows for the home page. "Rezepte" is an everyday German
// word that recipe sites rank for, so the name alone finds nothing; the words
// people actually search with have to be in the title. Other pages keep the
// short `%s — Rezepte` template.
export const SITE_TITLE = 'Rezepte — Self-hosted recipe manager for the people you cook with';

// The site-wide meta description, owned by the landing page (app/page.tsx).
export const SITE_DESCRIPTION =
	'Self-hosted, open-source recipe manager for your household and friends: one Docker image, SQLite, a cook mode for your phone, and an API and MCP server for AI assistants';

// Next merges metadata shallowly: a page-level `openGraph` REPLACES the root
// layout's rather than extending it. Every page that sets its own og:title has
// to spread these in, or it silently loses og:type, og:site_name and og:locale.
// The link preview, public/og.png (a copy of assets/brand/social-preview.png).
export const OG_IMAGES = [
	{ url: './og.png', width: 1280, height: 640, alt: 'The Rezepte mascot, a waving cookbook holding a wooden spoon, beside the name Rezepte' }
];

export const OG_SHARED = { type: 'website', siteName: 'Rezepte', locale: 'en_US', images: OG_IMAGES } as const;

const getContentUrl = createGetUrl('/llms.mdx');

export function getPageMarkdownUrl(page: { slugs: string[] }) {
	return getContentUrl([...page.slugs, 'content.md']);
}
