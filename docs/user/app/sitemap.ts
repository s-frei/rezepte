import type { MetadataRoute } from 'next';
import { source } from '@/lib/source';
import { SITE_URL } from '@/lib/shared';

// Required for `output: export`: sitemap.xml has no per-request data to revalidate.
export const dynamic = 'force-static';

export default function sitemap(): MetadataRoute.Sitemap {
	// `trailingSlash: true` (next.config.mjs) makes the slashed form canonical;
	// an entry without it 301s. `page.url` never carries one (root is "/",
	// everything else is "/foo/bar"), so add it back except for root.
	return source.getPages().map((page) => ({
		url: page.url === '/' ? SITE_URL : `${SITE_URL}${page.url.slice(1)}/`
	}));
}
