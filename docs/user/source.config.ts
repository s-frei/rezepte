import { defineConfig, defineDocs } from 'fumadocs-mdx/config';
import { remarkMdxMermaid } from 'fumadocs-core/mdx-plugins';

// End-user content lives in docs/user/content and is the only collection of
// this app; the agent memory is a separate site in docs/.
export const docs = defineDocs({
	dir: 'content',
	docs: {
		postprocess: { includeProcessedMarkdown: true }
	}
});

export default defineConfig({
	mdxOptions: {
		remarkPlugins: [remarkMdxMermaid]
	}
});
