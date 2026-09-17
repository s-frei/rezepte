import { createGetUrl } from 'fumadocs-core/source';

const getContentUrl = createGetUrl('/llms.mdx');

export function getPageMarkdownUrl(page: { slugs: string[] }) {
	return getContentUrl([...page.slugs, 'content.md']);
}
