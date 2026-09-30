/**
 * Where the user documentation is published (the `docs/user/` app, deployed
 * to GitHub Pages by `.github/workflows/pages.yml`). Every self-hosted
 * instance points at the same public guide, so this is a constant and not
 * configuration. The trailing slash matches the export's `trailingSlash: true`
 * in `docs/user/next.config.mjs`; without it Pages answers with a redirect.
 */
export const USER_GUIDE_URL = 'https://s-frei.github.io/rezepte/guide/';

/** The front page of the same site: what Rezepte is, for someone who never heard of it. */
export const PROJECT_URL = 'https://s-frei.github.io/rezepte/';

/** The release notes, on the same site. */
export const CHANGELOG_URL = 'https://s-frei.github.io/rezepte/changelog/';

/** Where the source code lives. */
export const SOURCE_URL = 'https://github.com/s-frei/rezepte';

/** The user-docs page on connecting an MCP client; same site as USER_GUIDE_URL. */
export const MCP_GUIDE_URL = 'https://s-frei.github.io/rezepte/api/mcp/';
