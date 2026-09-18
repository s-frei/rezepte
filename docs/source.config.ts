import { defineConfig, defineDocs } from 'fumadocs-mdx/config';
import { remarkMdxMermaid } from 'fumadocs-core/mdx-plugins';

// Content lives directly in docs/memory (not content/docs). `dir: '.'` with
// explicit globs keeps node_modules, app code and the separate user-docs app
// in docs/user out of this collection.
export const docs = defineDocs({
	dir: '.',
	docs: {
		files: ['memory/**/*.mdx'],
		postprocess: { includeProcessedMarkdown: true }
	},
	meta: {
		files: ['memory/**/meta.json']
	}
});

export default defineConfig({
	mdxOptions: {
		remarkPlugins: [remarkMdxMermaid]
	}
});
