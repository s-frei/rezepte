import './global.css';
import { RootProvider } from 'fumadocs-ui/provider/next';
import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { OG_IMAGES, OG_SHARED, SITE_TITLE, SITE_URL } from '@/lib/shared';
import { source } from '@/lib/source';

// The home page's description is the site's; content/index.mdx owns the wording.
const description = source.getPage([])?.data.description;

export const metadata: Metadata = {
	metadataBase: new URL(SITE_URL),
	title: { default: SITE_TITLE, template: '%s — Rezepte' },
	description,
	openGraph: { ...OG_SHARED, title: SITE_TITLE, description },
	twitter: { card: 'summary_large_image', images: OG_IMAGES }
};

// The static search client's default endpoint resolves via `import.meta.env.BASE_URL`,
// which Turbopack (Next 16's default builder, used here) polyfills per module from
// `basePath` - so it would in fact resolve correctly on its own. Pass the endpoint
// explicitly via `api` (which `DefaultSearchDialog` forwards to the static client's
// `from` option) anyway, so it does not depend on the builder polyfilling
// `import.meta.env` - Turbopack does, but a `--webpack` build or another builder might
// not. Do not read this as "simplify away, Next resolves it" - the point is not to
// depend on that.
const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? '';

export default function Layout({ children }: { children: ReactNode }) {
	return (
		<html lang="en" suppressHydrationWarning>
			<body className="flex min-h-screen flex-col">
				<RootProvider search={{ options: { type: 'static', api: `${basePath}/api/search` } }}>{children}</RootProvider>
			</body>
		</html>
	);
}
