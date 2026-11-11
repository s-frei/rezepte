import { expect, test, type Locator, type Page } from '@playwright/test';
import { login, uniqueToken } from './helpers';

// Below lg /forgot-password and /welcome open in the sheet, inside a frame that
// follows the visible area. Playwright raises no keyboard, so a short viewport
// stands in for the frame it leaves.

// The form shows whether or not this instance sends mail.
async function withResetAvailable(page: Page) {
	await page.route('**/api/v1/auth/password', (route) =>
		route.request().method() === 'GET'
			? route.fulfill({ json: { available: true } })
			: route.continue()
	);
}

async function documentScroll(page: Page) {
	return page.evaluate(() => {
		const el = document.scrollingElement!;
		return { top: el.scrollTop, fits: el.scrollHeight <= el.clientHeight };
	});
}

// Whether `el` sits inside the sheet's box, the scroller it lives in.
// toBeInViewport counts an element the sheet clips as in view. With a pixel
// to spare: Chromium rounds the scroll offset scrollIntoView sets to whole
// pixels, so a field can end a fraction past the sheet.
function inSheet(el: Locator) {
	return el.evaluate((node) => {
		const field = node.getBoundingClientRect();
		const box = node.closest('[data-auth-page]')!.getBoundingClientRect();
		return field.top >= box.top - 1 && field.bottom <= box.bottom + 1;
	});
}

test('forgot-password opens in the sheet and the page never scrolls', async ({
	page
}, testInfo) => {
	test.skip(testInfo.project.name !== 'mobile', 'the sheet is the phone layout');
	await withResetAvailable(page);
	await page.goto('/forgot-password');
	const sheet = page.locator('[data-auth-page]');
	await expect(sheet).toBeVisible();
	await expect(sheet.getByRole('heading', { name: 'Forgot password' })).toBeVisible();
	const field = page.getByLabel('Username or email');
	await field.fill('someone');
	expect(await documentScroll(page)).toEqual({ top: 0, fits: true });

	await page.setViewportSize({ width: 320, height: 400 });
	await field.focus();
	await expect.poll(() => inSheet(field)).toBe(true);
	await expect.poll(() => inSheet(page.getByRole('button', { name: 'Send link' }))).toBe(true);
	expect(await documentScroll(page)).toEqual({ top: 0, fits: true });
	await expect(sheet.getByRole('heading', { name: 'Forgot password' })).toBeVisible();
});

test('welcome opens in the sheet and scrolls inside it, never the page', async ({
	page,
	browser
}, testInfo) => {
	test.skip(testInfo.project.name !== 'mobile', 'the sheet is the phone layout');
	const name = `sheet${uniqueToken()}`;
	await login(page);
	await page.goto('/settings/users');
	await page.getByRole('button', { name: 'Add account' }).click();
	await page.getByLabel('Username').fill(name);
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	const url = await page
		.getByRole('dialog', { name: `Setup link for ${name}` })
		.getByRole('textbox')
		.inputValue();

	const invited = await (await browser.newContext({ ...testInfo.project.use })).newPage();
	await invited.goto(url);
	await expect(invited.getByRole('heading', { name: `Welcome, ${name}` })).toBeVisible();
	await invited.setViewportSize({ width: 360, height: 420 });
	const repeat = invited.getByLabel('Repeat the new password');
	await repeat.focus();
	await expect.poll(() => inSheet(repeat)).toBe(true);
	expect((await documentScroll(invited)).top).toBe(0);
	const sheet = invited.locator('[data-auth-page]');
	expect(await sheet.evaluate((el) => el.scrollHeight > el.clientHeight)).toBe(true);
	// Too tall for the frame, it scrolls rather than giving anything up:
	// the heading stays as it is.
	const heading = invited.getByRole('heading', { name: `Welcome, ${name}` });
	await sheet.evaluate((el) => el.scrollTo(0, 0));
	await expect(heading).toBeVisible();
	expect(await heading.evaluate((el) => getComputedStyle(el).position)).toBe('static');

	// The frame changing size brings the focused field back into view: what
	// the keyboard rising does on a phone. Focusing alone would not prove it,
	// since the browser scrolls a field into view on focus by itself.
	// 340px, so that scrolled to its top the sheet hides the field.
	const shown = () => inSheet(repeat);
	await invited.setViewportSize({ width: 360, height: 340 });
	await expect.poll(shown).toBe(true);
	// Scrolled again on every try: the resize may still bring the field back.
	await expect
		.poll(async () => {
			await sheet.evaluate((el) => el.scrollTo(0, 0));
			return shown();
		})
		.toBe(false);
	await invited.setViewportSize({ width: 360, height: 330 });
	await expect.poll(shown).toBe(true);
	expect((await documentScroll(invited)).top).toBe(0);
});

test('from lg forgot-password keeps the card, no frame', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name !== 'desktop', 'the card is the desktop layout');
	await withResetAvailable(page);
	await page.goto('/forgot-password');
	await expect(page.locator('[data-auth-card]')).toBeVisible();
	expect(await page.locator('main').evaluate((el) => getComputedStyle(el).position)).not.toBe(
		'fixed'
	);
});
