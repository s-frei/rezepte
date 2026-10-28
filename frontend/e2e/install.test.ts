import { expect, test } from '@playwright/test';
import { login } from './helpers';

test('Chromium finds the app installable', async ({ page, isMobile }) => {
	test.skip(isMobile, 'the installability audit is a desktop Chromium call');
	await login(page);
	const cdp = await page.context().newCDPSession(page);
	const { installabilityErrors } = await cdp.send('Page.getInstallabilityErrors');
	expect(installabilityErrors).toEqual([]);
});
