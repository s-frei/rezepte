import './global.css';
import { RootProvider } from 'fumadocs-ui/provider/next';
import { ReleaseBanner } from '@/components/releases';
import { SearchDialog } from '@/components/search';
import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { OG_IMAGES, OG_SHARED, SITE_DESCRIPTION as description, SITE_TITLE, SITE_URL } from '@/lib/shared';

export const metadata: Metadata = {
	metadataBase: new URL(SITE_URL),
	title: { default: SITE_TITLE, template: '%s — Rezepte' },
	description,
	openGraph: { ...OG_SHARED, title: SITE_TITLE, description },
	twitter: { card: 'summary_large_image', images: OG_IMAGES }
};

// The search dialog lives in components/search.tsx.
export default function Layout({ children }: { children: ReactNode }) {
	return (
		<html lang="en" suppressHydrationWarning>
			<body className="flex min-h-screen flex-col">
				<RootProvider search={{ SearchDialog }}>
					<ReleaseBanner />
					{children}
				</RootProvider>
			</body>
		</html>
	);
}
