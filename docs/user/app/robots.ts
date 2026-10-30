import type { MetadataRoute } from 'next';
import { SITE_URL } from '@/lib/shared';

// Required for `output: export`: robots.txt has no per-request data to revalidate.
export const dynamic = 'force-static';

// Exports to the origin root, the only place crawlers read robots.txt, and points
// them at the sitemap.
export default function robots(): MetadataRoute.Robots {
	return {
		rules: { userAgent: '*', allow: '/' },
		sitemap: `${SITE_URL}sitemap.xml`
	};
}
