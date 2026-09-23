// Generates content/api/reference/ from public/openapi.json: one page per
// operation, grouped by tag, plus an index page per group and one over all of
// them. Never edit the output by hand; regenerate with
// `mise run //docs/user:openapi`.
import { readFileSync } from 'node:fs';
import { generateFiles } from 'fumadocs-openapi';
import { openapi } from '../lib/openapi';

const OUTPUT = './content/api/reference';

// `groupBy: 'tag'` groups by the tags an operation carries, so the groups come
// out in the order the paths appear in the document. The order the tags are
// *declared* in is the editorial one (service/internal/httpserver/api.go), and
// it is restored onto the top-level meta.json below.
const doc = JSON.parse(readFileSync('public/openapi.json', 'utf8')) as { tags?: { name: string }[] };
const tagOrder = (doc.tags ?? []).map((tag) => tag.name);

/** `<dir>/<file>.mdx` -> the URL Fumadocs serves it at. */
const urlOf = (filePath: string) => `/api/reference/${filePath.replace(/\.mdx$/, '')}`;

await generateFiles({
	input: openapi,
	output: OUTPUT,
	// A page per operation, foldered by tag. The sidebar then lists the
	// endpoints themselves, each carrying its method badge (the badge comes
	// from `openapiPlugin()` in lib/source.ts, which reads `_openapi.method` -
	// a field only operation pages have).
	per: 'operation',
	groupBy: 'tag',
	// Left off deliberately: it moves the description into the page body, and
	// every other page on this site carries its description in the frontmatter,
	// where <DocsDescription>, the search index and /llms.txt all read it.
	includeDescription: false,
	index: {
		// One card page per tag, plus one over all of them. Without the group
		// pages a tag is a bare navigation node: `/api/reference/recipes` would
		// 404 and the breadcrumb above an operation would lead nowhere.
		items({ generatedEntries }) {
			const groups = Object.values(generatedEntries)
				.flat()
				.filter((entry) => entry.type === 'group');
			return [
				{
					path: 'index.mdx',
					title: 'API reference',
					description: 'Every endpoint of the HTTP API, generated from the OpenAPI document.'
				},
				...groups.map((group) => ({
					path: `${group.path}/index.mdx`,
					title: group.info.title,
					description: group.info.description,
					only: group.entries.map((entry) => entry.path)
				}))
			];
		},
		url: urlOf
	},
	meta: true,
	beforeWrite(files) {
		// The sidebar and the page heading should read as the endpoint they
		// are, not as a sentence about it. `frontmatter` cannot do this: it is
		// handed the title and description of an entry, never the route. Here
		// the entries are still available, so map each output file back to the
		// operation it came from and swap the two around - the route becomes
		// the title, the summary becomes the description.
		const routes = new Map<string, string>();
		const collect = (entries: { type: string; path: string; [k: string]: unknown }[]) => {
			for (const entry of entries) {
				if (entry.type === 'group') {
					collect(entry.entries as never);
				} else if (entry.type === 'operation') {
					routes.set(entry.path, (entry.item as { path: string }).path);
				}
			}
		};
		collect(Object.values(this.generatedEntries).flat() as never);

		for (const file of files) {
			const route = routes.get(file.path);
			if (!route) continue;
			// An operation that carries its own description already has the
			// key; only one without it inherits the summary, or the frontmatter
			// ends up with two `description` keys and the YAML no longer parses.
			const described = /^description:/m.test(file.content.split('---')[1] ?? '');
			// Frontmatter is generated, so `title:` is always the first key
			// after the opening `---`.
			file.content = file.content.replace(/^---\ntitle: (.*)$/m, (_match, summary: string) =>
				described ? `---\ntitle: '${route}'` : `---\ntitle: '${route}'\ndescription: ${summary}`
			);
		}

		// `generateMeta` only walks the pages it made itself, so every index
		// page it does not know about would be missing from the sidebar. The
		// top-level one is additionally put back into declaration order.
		for (const file of files) {
			if (!file.path.endsWith('meta.json')) continue;
			const parsed = JSON.parse(file.content) as { pages: string[] };
			const rest = parsed.pages.filter((page) => page !== 'index');
			parsed.pages =
				file.path === 'meta.json'
					? [
							'index',
							...tagOrder.filter((tag) => rest.includes(tag)),
							...rest.filter((page) => !tagOrder.includes(page))
						]
					: ['index', ...rest];
			file.content = `${JSON.stringify(parsed, null, 2)}\n`;
		}
	}
});
