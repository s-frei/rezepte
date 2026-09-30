'use client';
import { createOpenAPIPage } from 'fumadocs-openapi/ui';

// The playground is off, for two independent reasons:
//
// - This app is a static export, so `openapi.createProxy()` - a route handler -
//   cannot exist, and the playground would have to reach the target instance
//   straight from the browser.
// - The Go service sends no CORS headers, so that cross-origin request fails
//   from the published domain to any self-hosted instance.
//
// A playground that reaches no instance is worse than none. The app serves
// Scalar at /api/v1/docs, same-origin, where it does reach one.

// The import takes a zip as its body. The renderer refuses a media type it
// has no adapter for; this one shows the schema and skips the code samples.
export const OpenAPIPage = createOpenAPIPage({
	playground: { enabled: false },
	mediaAdapters: {
		'application/zip': { encode: (data) => data.body as BodyInit, generateExample: () => undefined }
	}
});
