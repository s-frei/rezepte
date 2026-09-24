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
		remarkPlugins: [remarkMdxMermaid],
		// Fumadocs defaults to github-light/github-dark, whose cool grays and
		// blues sit oddly on the warm paper of the Rezepte palette. Vitesse is
		// muted and warm-leaning; only its syntax colors are used, because the
		// code block's surface comes from `fd-card`. `defaultColor: false` is
		// part of the default and has to be repeated: passing `themes` replaces
		// the defaults wholesale, and without it Shiki emits fixed colors
		// instead of the --shiki-light/--shiki-dark variables that dark mode
		// reads.
		rehypeCodeOptions: {
			themes: { light: 'vitesse-light', dark: 'vitesse-dark' },
			defaultColor: false
		}
	}
});
