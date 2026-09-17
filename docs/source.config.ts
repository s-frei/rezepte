import { defineConfig, defineDocs } from 'fumadocs-mdx/config';
import { remarkMdxMermaid } from 'fumadocs-core/mdx-plugins';

// Content lives directly in docs/memory and docs/user (not content/docs).
// `dir: '.'` with explicit globs keeps node_modules and app code out.
export const docs = defineDocs({
	dir: '.',
	docs: {
		files: ['memory/**/*.mdx', 'user/**/*.mdx'],
		postprocess: { includeProcessedMarkdown: true }
	},
	meta: {
		files: ['memory/**/meta.json', 'user/**/meta.json']
	}
});

export default defineConfig({
	mdxOptions: {
		remarkPlugins: [remarkMdxMermaid]
	}
});
