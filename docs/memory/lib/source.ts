import { loader, llms } from 'fumadocs-core/source';
// The installed fumadocs-mdx (15.4.1) generates .source/server.ts (no plain
// .source/index.ts), so the collections are imported from '@/.source/server'.
import { docs } from '@/.source/server';

export const source = loader({
	baseUrl: '/',
	source: docs.toFumadocsSource()
});

export const docsLlms = llms(source, {
	renderPage: async (page) =>
		`# ${page.data.title} (${page.url})\n\n${await page.data.getText('processed')}`
});
