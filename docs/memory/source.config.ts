import { defineConfig, defineDocs } from 'fumadocs-mdx/config';
import { remarkMdxMermaid } from 'fumadocs-core/mdx-plugins';

// Agent-memory content lives in docs/memory/content and is the only
// collection of this app; the end-user docs are a separate site in
// docs/user.
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
