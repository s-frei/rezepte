import type { MetadataRoute } from 'next';
import { SITE_URL } from '@/lib/shared';

// Required for `output: export`: robots.txt has no per-request data to revalidate.
export const dynamic = 'force-static';

// This exports to /rezepte/robots.txt, which crawlers never read - robots.txt is only
// honored at the origin root, and that origin (s-frei.github.io) belongs to a
// different repository. Kept anyway: it costs nothing, becomes correct if this site
// ever moves to a custom domain, and documents intent. Until then the sitemap has to
// be submitted to search engines manually (see
// docs/memory/content/architecture/docs.mdx).
export default function robots(): MetadataRoute.Robots {
	return {
		rules: { userAgent: '*', allow: '/' },
		sitemap: `${SITE_URL}sitemap.xml`
	};
}
