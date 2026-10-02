import { loader, llms } from 'fumadocs-core/source';
import { openapiPlugin } from 'fumadocs-openapi/server';
// The installed fumadocs-mdx (15.4.1) generates .source/server.ts (no plain
// .source/index.ts), so the collections are imported from '@/.source/server'.
import { docs } from '@/.source/server';
import { SITE_URL } from '@/lib/shared';
import { ROLE_LABEL } from '@/components/audience';

// Pages generated from the OpenAPI document live under this prefix. They are
// real files in content/api/reference/, but their body is a single
// <OpenAPIPage> tag, so anything that wants text out of them needs the branch
// in `renderPage` below rather than the page itself.
const API_REFERENCE_PREFIX = '/api/reference';

export const source = loader({
	baseUrl: '/',
	source: docs.toFumadocsSource(),
	// Adds the HTTP method badge to each generated page in the page tree.
	plugins: [openapiPlugin()]
});

// The loader's URLs are root-relative ("/guide"), which a browser resolves
// under the /rezepte base path but an assistant reading /llms.txt does not:
// it lands on s-frei.github.io/guide. Everything the llms routes serve goes
// through this, so a Markdown link always names the full address.
export function absoluteLinks(markdown: string): string {
	return markdown.replaceAll('](/', `](${SITE_URL}`);
}

export const docsLlms = llms(source, {
	renderPage: async (page) => {
		// fumadocs-openapi renders through a React component, so a generated
		// page carries no prose: `getText('processed')` would return the
		// component call, not the endpoints. Name the page and point at the
		// document instead - it is the machine-readable form anyway, and it is
		// what a reader of these routes actually wants.
		if (page.url.startsWith(API_REFERENCE_PREFIX)) {
			return [
				`# ${page.data.title} (${SITE_URL}${page.url.slice(1)})`,
				'',
				`The endpoints of this page are defined in the OpenAPI document: ${SITE_URL}openapi.json`,
				''
			].join('\n');
		}
		// A changelog page's upgrade notice is frontmatter, rendered by page.tsx;
		// without this line the Markdown view would silently drop it.
		const upgrade = page.data.upgrade ? `> **Before you upgrade:** ${page.data.upgrade}\n\n` : '';
		// The audience is a badge in the page chrome, which the Markdown view has
		// none of; a line keeps it.
		const audience = page.data.audience ? `> **${ROLE_LABEL[page.data.audience]}.**\n\n` : '';
		const text = `${audience}${upgrade}${await page.data.getText('processed')}`;
		return `# ${page.data.title} (${SITE_URL}${page.url.slice(1)})\n\n${absoluteLinks(text)}`;
	}
});
