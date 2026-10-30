import type { MetadataRoute } from 'next';
import { source } from '@/lib/source';
import { publishedUrl } from '@/lib/shared';

// Required for `output: export`: sitemap.xml has no per-request data to revalidate.
export const dynamic = 'force-static';

export default function sitemap(): MetadataRoute.Sitemap {
	// The landing page at the root is not a content page, so it is listed first.
	return [{ url: publishedUrl('/') }, ...source.getPages().map((page) => ({ url: publishedUrl(page.url) }))];
}
