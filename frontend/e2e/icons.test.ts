import { expect, test } from '@playwright/test';

// A missing file is answered by the SPA fallback with 200 and index.html,
// so every check pins the content type, not only the status.
const links: [selector: string, type: string][] = [
	['link[rel="icon"][type="image/svg+xml"]', 'image/svg+xml'],
	['link[rel="icon"][sizes="32x32"]', 'image/'],
	['link[rel="apple-touch-icon"]', 'image/png'],
	['link[rel="manifest"]', 'application/manifest+json']
];

for (const [selector, type] of links) {
	test(`head links ${selector} and it loads`, async ({ page, request }) => {
		await page.goto('/login');
		const href = await page.locator(selector).getAttribute('href');
		expect(href).toBeTruthy();
		const res = await request.get(new URL(href!, page.url()).pathname);
		expect(res.status()).toBe(200);
		expect(res.headers()['content-type']).toContain(type);
	});
}

test('the head carries a link preview whose image loads', async ({ page, request }) => {
	await page.goto('/login');
	const meta = (key: string) => page.locator(`meta[property="${key}"], meta[name="${key}"]`);
	await expect(meta('og:title')).toHaveAttribute('content', 'Rezepte');
	await expect(meta('og:description')).toHaveAttribute('content', /recipe/i);
	await expect(meta('twitter:card')).toHaveAttribute('content', 'summary_large_image');
	const image = await meta('og:image').getAttribute('content');
	const res = await request.get(new URL(image!, page.url()).pathname);
	expect(res.headers()['content-type']).toBe('image/png');
});

test('every manifest icon loads as a png', async ({ request }) => {
	const manifest = (await (await request.get('/manifest.webmanifest')).json()) as {
		icons: { src: string }[];
	};
	for (const icon of manifest.icons) {
		const res = await request.get(icon.src);
		expect(res.headers()['content-type'], icon.src).toBe('image/png');
	}
});
