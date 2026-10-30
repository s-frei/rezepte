import { createFromSource } from 'fumadocs-core/search/server';
import { sectionOf } from '@/lib/sections';
import { API_REFERENCE_PREFIX, source } from '@/lib/source';

export const revalidate = false;
export const { staticGET: GET } = createFromSource(source, {
	language: 'english',
	// The default index plus the page's section, which the dialog filters by.
	buildIndex: async (page) => ({
		title: page.data.title,
		description: page.data.description,
		url: page.url,
		id: page.url,
		// A generated reference page holds no prose, only component props (its
		// cards and operation), which the index would show as raw "card href:"
		// text; its title and description are what a search should find.
		structuredData: page.url.startsWith(API_REFERENCE_PREFIX) ? { headings: [], contents: [] } : page.data.structuredData,
		tag: sectionOf(page.url)
	})
});
