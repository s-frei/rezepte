/**
 * Where the user documentation is published (the `docs/user/` app, deployed
 * to GitHub Pages by `.github/workflows/pages.yml`). Every self-hosted
 * instance points at the same public guide, so this is a constant and not
 * configuration. The trailing slash matches the export's `trailingSlash: true`
 * in `docs/user/next.config.mjs`; without it Pages answers with a redirect.
 */
export const USER_GUIDE_URL = 'https://s-frei.github.io/rezepte/guide/';
