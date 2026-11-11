import { expect, test, type Page } from '@playwright/test';
import {
	createUser,
	devPassword,
	login,
	openLoginSheet,
	pinLocale,
	setOwnAvatarViaApi,
	signOut,
	tinyPng,
	uniqueToken
} from './helpers';
import { USER_THEME_COLORS } from '../src/lib/user/theme-color';

test('redirects anonymous users to login', async ({ page }) => {
	await page.goto('/');
	await expect(page).toHaveURL(/\/login\?next=%2F/);
});

// SvelteKit 3's goto rejects a path without a route, and the login form read
// that rejection as a failed sign-in: signed in, yet told it went wrong.
test('a sign-in towards a page that does not exist lands on its 404', async ({ page }) => {
	await pinLocale(page);
	await page.goto('/nope');
	await expect(page).toHaveURL('/login?next=%2Fnope');
	// The form, not the API: the redirect after it is what broke. A lock is
	// the throttle race the login helper describes, so it waits and retries.
	await openLoginSheet(page);
	for (let attempt = 0; attempt < 6; attempt++) {
		await page.getByLabel('Username').fill('admin');
		await page.getByLabel('Password').fill(devPassword('admin'));
		await page.getByRole('button', { name: 'Sign in' }).click();
		const outcome = await Promise.race([
			page.waitForURL('/nope').then(() => 'in'),
			page
				.getByRole('alert')
				.waitFor()
				.then(async () => page.getByRole('alert').textContent())
		]);
		if (outcome === 'in' || !outcome?.startsWith('Too many')) break;
		await page.waitForTimeout(250 * (attempt + 1));
	}
	await expect(page).toHaveURL('/nope');
	await expect(page.getByRole('heading', { name: "This page isn't in the book" })).toBeVisible();
	await expect(page.getByRole('alert')).toHaveCount(0);
});

// A failed session load (the server unreachable) fails the root layout, which
// SvelteKit would answer with its bare built-in page; ours offers a retry. An
// unknown path takes another way through SvelteKit (+layout.ts), so both.
test('an unreachable server shows the error page, and trying again recovers', async ({ page }) => {
	await pinLocale(page);
	await login(page);
	await page.route('**/api/v1/auth/me', (route) => route.abort());
	for (const path of ['/', '/nope']) {
		await page.goto(path);
		await expect(page.getByRole('heading', { name: 'Something went wrong' })).toBeVisible();
	}
	await page.unroute('**/api/v1/auth/me');
	await page.getByRole('button', { name: 'Try again' }).click();
	await expect(page.getByRole('heading', { name: "This page isn't in the book" })).toBeVisible();
	await page.getByRole('link', { name: 'Back to recipes' }).click();
	await expect(page.getByRole('heading', { name: 'Something went wrong' })).toHaveCount(0);
	await expect(page).toHaveURL('/');
});

test('rejects a wrong password', async ({ page }) => {
	await login(page, 'admin', 'wrong');
	await expect(page.getByRole('alert')).toHaveText('That username or password is wrong.');
	await expect(page).toHaveURL(/\/login/);
});

test('explains a sign-in locked after too many failures', async ({ page }, testInfo) => {
	// A name of its own per project and run, so the lock never reaches the
	// accounts other specs sign in with. Unknown names are locked like real ones.
	const username = `locked-${testInfo.project.name}-${Date.now()}`;
	for (let attempt = 0; attempt < 5; attempt++) {
		await login(page, username, 'wrong');
		await expect(page.getByRole('alert')).toBeVisible();
	}
	await login(page, username, 'wrong');
	await expect(page.getByRole('alert')).toHaveText(
		'Too many failed attempts. Wait a little, then try again.'
	);
});

test('logs in, survives reload, logs out', async ({ page }, testInfo) => {
	await login(page);
	// Being on the overview is the proof: an anonymous visitor is bounced to
	// /login by the session guard, so the headline can only render signed in.
	await expect(page).toHaveURL('/');
	const headline = page.getByRole('heading', { name: 'What are we cooking today?' });
	await expect(headline).toBeVisible();
	await page.reload();
	await expect(page).toHaveURL('/');
	await expect(headline).toBeVisible();
	await signOut(page, testInfo);
	await expect(page).toHaveURL(/\/login/);
});

// A password manager fills and saves by these attributes alone, and a phone
// keyboard that capitalized "sam" to "Sam" would fail the login.
test('the login form speaks to password managers', async ({ page }) => {
	await page.goto('/login');
	const username = page.getByLabel('Username');
	await expect(username).toHaveAttribute('autocomplete', 'username');
	await expect(username).toHaveAttribute('autocapitalize', 'none');
	await expect(username).toHaveAttribute('spellcheck', 'false');
	await expect(page.getByLabel('Password')).toHaveAttribute('autocomplete', 'current-password');
});

// The page has one job, so it saves the click into its first field.
test('the login form starts in the username field', async ({ page }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'the phone opens on a cover');
	await page.goto('/login');
	await expect(page.getByLabel('Username')).toBeFocused();
});

test('the password change names the account it belongs to', async ({ page }) => {
	await login(page);
	await page.goto('/settings');
	await page.getByRole('button', { name: 'Change password' }).click();
	const account = page.locator('input[autocomplete="username"]');
	await expect(account).toHaveValue('admin');
	await expect(account).toBeHidden();
	await expect(page.getByLabel('New password', { exact: true })).toHaveAttribute(
		'autocomplete',
		'new-password'
	);
});

test("the browser's bar wears the signed-in person's color until sign-out", async ({
	page
}, testInfo) => {
	const bar = (scheme: 'light' | 'dark') =>
		page.locator(`meta[name="theme-color"][data-scheme="${scheme}"]`);
	await page.goto('/login');
	await expect(bar('light')).toHaveAttribute('content', '#f4ede2');

	await login(page);
	const me = (await (await page.request.get('/api/v1/auth/me')).json()) as {
		color: keyof typeof USER_THEME_COLORS;
	};
	await expect(bar('light')).toHaveAttribute('content', USER_THEME_COLORS[me.color].light);
	await expect(bar('dark')).toHaveAttribute('content', USER_THEME_COLORS[me.color].dark);

	await signOut(page, testInfo);
	await expect(page).toHaveURL(/\/login/);
	await expect(bar('light')).toHaveAttribute('content', '#f4ede2');
	await expect(bar('dark')).toHaveAttribute('content', '#1c1612');
});

test('the login page says once who signed out, by initial and name', async ({
	page,
	browser,
	baseURL
}, testInfo) => {
	const name = `bye${uniqueToken()}`.toLowerCase();
	// The account is made from a browser of its own: signing admin in on this
	// page too would leave admin on the device, and sign-out would then lead
	// to the picker instead of the login.
	const owner = await browser.newContext({ baseURL });
	const ownerPage = await owner.newPage();
	await login(ownerPage);
	await createUser(ownerPage, { username: name, role: 'user' });
	await owner.close();
	await login(page, name);
	// A photo, which the public login page must not show.
	await setOwnAvatarViaApi(page, tinyPng([30, 140, 90]));
	await signOut(page, testInfo);
	await expect(page).toHaveURL(/\/login/);
	const notice = page.getByRole('status').filter({ hasText: `Signed out: ${name}` });
	await expect(notice).toBeVisible();
	await expect(notice.getByText(name.charAt(0).toUpperCase(), { exact: true })).toBeVisible();
	await expect(notice.locator('img')).toHaveCount(0);
	if (testInfo.project.name.startsWith('mobile')) {
		// The line stands on the cover, outside the sheet.
		await expect(page.locator('[data-auth-cover]').getByRole('status')).toContainText(name);
		await expect(page.getByLabel('Username')).toBeHidden();
		await openLoginSheet(page);
	}
	await expect(page.getByLabel('Username')).toHaveValue('');

	await page.reload();
	await openLoginSheet(page);
	await expect(page.getByLabel('Username')).toBeVisible();
	await expect(page.getByText(`Signed out: ${name}`)).toHaveCount(0);
});

// The login and forgot-password frames hide the document's overflow while
// they are mounted; a sign-in after a visit to the other must not carry it
// into the app, where nothing could scroll.
test('the app scrolls after a sign-in that went by forgot-password', async ({ page }) => {
	await pinLocale(page);
	await page.route('**/api/v1/auth/password', (route) =>
		route.request().method() === 'GET'
			? route.fulfill({ json: { available: true } })
			: route.continue()
	);
	await page.goto('/login');
	await openLoginSheet(page);
	await page.getByRole('link', { name: 'Forgot password?' }).click();
	await expect(page).toHaveURL('/forgot-password');
	await page.goBack();
	await expect(page).toHaveURL('/login');
	await openLoginSheet(page);
	// Exact: the address reads /login while forgot-password's "Username or
	// email" may still be on screen, and a loose label filled that one.
	await page.getByLabel('Username', { exact: true }).fill('admin');
	await page.getByLabel('Password').fill(devPassword('admin'));
	await page.getByRole('button', { name: 'Sign in' }).click();
	await expect(page.getByRole('heading', { name: 'What are we cooking today?' })).toBeVisible();
	expect(await page.evaluate(() => document.documentElement.style.overflow)).toBe('');
});

// Below lg the login opens on a cover, and the form waits in a sheet behind it.
// A sign-in ends as a switch does (#lib/arrival.ts): the form grows into the
// greeting in the person's color, and the reload closes that color into the
// avatar. A person of their own, so the form meets no throttle race on admin.
test.describe('the arrival after a sign-in', () => {
	async function signInAsNew(page: Page) {
		await login(page);
		const username = `mila${uniqueToken()}`;
		await createUser(page, { username, role: 'user', displayName: 'Mila Probe' });
		await page.context().clearCookies();
		await pinLocale(page);
		await page.goto('/login');
		await openLoginSheet(page);
		await page.getByLabel('Username').fill(username);
		await page.getByLabel('Password').fill(devPassword(username));
		// Kept over the reload in this tab: whether the greeting ever showed.
		await page.evaluate(() =>
			new MutationObserver(() => {
				if (document.body.textContent?.includes('Hello, Mila!'))
					sessionStorage.setItem('saw-hello', '1');
			}).observe(document.body, { subtree: true, childList: true })
		);
		await page.getByRole('button', { name: 'Sign in' }).click();
	}

	test('greets by first name, then closes into the avatar', async ({ page }) => {
		// `closing` lasts 450ms from the reload's first frames, which a check
		// after `waitForURL` can miss: each document records it as it comes.
		await page.addInitScript(() =>
			// The document, not <html>: an init script can run before it exists.
			new MutationObserver(() => {
				if (document.documentElement?.dataset.arrival === 'closing')
					sessionStorage.setItem('saw-closing', '1');
			}).observe(document, { subtree: true, attributeFilter: ['data-arrival'] })
		);
		await signInAsNew(page);
		await expect(page.getByRole('status').filter({ hasText: 'Hello, Mila!' })).toBeVisible();
		await page.waitForURL('/');
		const html = page.locator('html');
		await expect(html).not.toHaveAttribute('data-arrival', { timeout: 2500 });
		expect(await page.evaluate(() => sessionStorage.getItem('saw-closing'))).toBe('1');
		// The watch the reduced-motion test relies on saw it too.
		expect(await page.evaluate(() => sessionStorage.getItem('saw-hello'))).toBe('1');
	});

	test.describe('under reduced motion', () => {
		test.use({ contextOptions: { reducedMotion: 'reduce' } });

		test('neither greets nor arrives', async ({ page }) => {
			await signInAsNew(page);
			await page.waitForURL('/');
			await page.getByRole('main').waitFor();
			await expect(page.locator('html')).not.toHaveAttribute('data-arrival');
			expect(await page.evaluate(() => sessionStorage.getItem('saw-hello'))).toBeNull();
		});
	});
});

test.describe('the phone login', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		test.skip(!testInfo.project.name.startsWith('mobile'), 'the cover is phone only');
		await pinLocale(page);
	});

	const cover = (page: Page) => page.locator('[data-auth-cover]');
	const coverButton = (page: Page) => cover(page).getByRole('button', { name: 'Sign in' });
	const sheet = (page: Page) => page.getByRole('dialog', { name: 'Sign in' });

	test('the phone login opens on a cover with no field focused', async ({ page }) => {
		await page.goto('/login');
		await expect(coverButton(page)).toBeVisible();
		await expect(page.getByLabel('Username')).toBeHidden();
		expect(await page.evaluate(() => document.activeElement === document.body)).toBe(true);
	});

	test('tapping Sign in raises the sheet with the username focused', async ({ page }) => {
		await page.goto('/login');
		await expect(coverButton(page)).toBeVisible();
		// Click and read the focus in one synchronous call: iOS raises the
		// keyboard only for a focus inside the tap, and a poll would pass a late one.
		const focused = await page.evaluate(() => {
			document.querySelector<HTMLElement>('[data-auth-cover]')?.querySelector('button')?.click();
			return document.activeElement?.id;
		});
		expect(focused).toBe('username');
		await expect(sheet(page)).toBeVisible();
		await expect(page.locator('[data-auth-page][role="dialog"][aria-modal="true"]')).toBeVisible();
	});

	test('Enter in the username moves to the password', async ({ page }) => {
		await page.goto('/login');
		await openLoginSheet(page);
		await page.getByLabel('Username').fill('admin');
		await page.keyboard.press('Enter');
		await expect(page.getByLabel('Password')).toBeFocused();
		await expect(page).toHaveURL(/\/login$/);
		await expect(page.getByRole('alert')).toHaveCount(0);
	});

	test('the sheet closes on Escape and on a tap on the painting', async ({ page }) => {
		await page.goto('/login');
		await openLoginSheet(page);
		const username = page.locator('#username');
		const before = await username.elementHandle();
		const closers = [
			() => page.keyboard.press('Escape'),
			() => page.locator('[data-auth-plate]').click({ position: { x: 20, y: 20 } })
		];
		for (const close of closers) {
			await close();
			await expect(sheet(page)).toBeHidden();
			await expect(coverButton(page)).toBeFocused();
			await coverButton(page).click();
			await expect(page.getByLabel('Username')).toBeFocused();
			// The same node, so what was typed and the password manager's fill survive.
			const after = await username.elementHandle();
			expect(await page.evaluate(([a, b]) => a === b, [before, after])).toBe(true);
		}
	});

	test('a wrong password shows the error inside the sheet and the page does not scroll', async ({
		page
	}) => {
		await page.goto('/login');
		await openLoginSheet(page);
		await page.getByLabel('Username').fill(`nobody${uniqueToken()}`);
		await page.getByLabel('Password').fill('wrong');
		await page.getByRole('button', { name: 'Sign in' }).click();
		const alert = sheet(page).getByRole('alert');
		await expect(alert).toHaveText('That username or password is wrong.');
		await expect(page.locator('#password')).toBeFocused();
		const scroll = await page.evaluate(() => {
			const el = document.scrollingElement!;
			return { top: el.scrollTop, over: el.scrollHeight - el.clientHeight };
		});
		expect(scroll.top).toBe(0);
		expect(scroll.over).toBeLessThanOrEqual(0);
	});

	test('the grip closes the sheet on a tap and on a pull', async ({ page }) => {
		await page.goto('/login');
		await openLoginSheet(page);
		const grip = sheet(page).getByRole('button', { name: 'Close' });
		await grip.click();
		await expect(sheet(page)).toBeHidden();
		await expect(coverButton(page)).toBeFocused();

		await coverButton(page).click();
		await expect(sheet(page)).toBeVisible();
		// Let the sheet finish rising, or the grip moves under the finger.
		await page
			.locator('[data-auth-page]')
			.evaluate((el) => Promise.all(el.getAnimations().map((a) => a.finished)));
		const box = (await grip.boundingBox())!;
		const x = box.x + box.width / 2;
		let y = box.y + box.height / 2;
		await page.mouse.move(x, y);
		await page.mouse.down();
		for (let step = 0; step < 10; step++) {
			y += 30;
			await page.mouse.move(x, y);
		}
		await page.mouse.up();
		await expect(sheet(page)).toBeHidden();
		await expect(coverButton(page)).toBeFocused();
		// The pull's transform is gone with the sheet, not left on the cover.
		await expect(page.locator('[data-auth-page]')).toHaveCSS('transform', 'none');
	});

	// Every move of the frame's contents, then a frame for the ResizeObserver.
	const settled = (page: Page) =>
		page.evaluate(async () => {
			await new Promise(requestAnimationFrame);
			await Promise.all(document.getAnimations().map((a) => a.finished.catch(() => {})));
		});

	test('with the painting gone the heading wears the mascot and the lead', async ({ page }) => {
		await page.goto('/login');
		await openLoginSheet(page);
		const main = page.locator('main');
		const heading = page.locator('#login-title');
		const header = () =>
			heading.evaluate((el) => ({
				mascot: getComputedStyle(el, '::before').width,
				image: getComputedStyle(el, '::before').backgroundImage,
				lead: getComputedStyle(el, '::after').content,
				leadShown: getComputedStyle(el, '::after').opacity
			}));
		await expect(main).not.toHaveAttribute('data-flat');
		await settled(page);
		expect(await header()).toMatchObject({ mascot: '0px', leadShown: '0' });

		// The keyboard's stand-in: too short for the painting.
		await page.getByLabel('Username').focus();
		await page.setViewportSize({ width: 360, height: 468 });
		await expect(main).toHaveAttribute('data-flat');
		await settled(page);
		expect(await header()).toEqual({
			mascot: '56px',
			image: expect.stringContaining('rezepte-mascot'),
			lead: '"Welcome back." / ""',
			leadShown: '1'
		});
		// Decorative: the dialog is still named by the heading alone.
		await expect(sheet(page)).toBeVisible();
	});

	// Gboard adds its number row in a password, so the keyboard changes
	// height between the two fields: that only moves the sheet, never what is
	// in it, and a plate collapsed while typing stays collapsed. It starts
	// too flat and grows to where the plate would fit again (700px): from a
	// height that stays too flat anyway, the test would pass without the
	// stickiness it is there for.
	test('a keyboard changing height while typing moves the sheet, never its contents', async ({
		page
	}) => {
		await page.goto('/login');
		await openLoginSheet(page);
		const main = page.locator('main');
		await page.getByLabel('Username').focus();
		await page.setViewportSize({ width: 360, height: 470 });
		await expect(main).toHaveAttribute('data-flat');
		await settled(page);
		// The card's insides, relative to the card's top.
		const layout = () =>
			page.locator('[data-auth-card]').evaluate((card) => {
				const top = card.getBoundingClientRect().top;
				const at = (q: string) => {
					const r = card.querySelector(q)!.getBoundingClientRect();
					return [r.top - top, r.height];
				};
				const h1 = card.querySelector('h1')!;
				return {
					heading: at('h1'),
					username: at('#username'),
					password: at('#password'),
					submit: at('button[type="submit"]'),
					lead: getComputedStyle(h1, '::after').opacity
				};
			});
		const before = await layout();
		expect(before.lead).toBe('1');

		await page.getByLabel('Password').focus();
		await page.setViewportSize({ width: 360, height: 700 });
		await settled(page);
		await expect(main).toHaveAttribute('data-flat');
		expect(await layout()).toEqual(before);

		// Focus gone, the plate comes back where it fits, with no resize.
		await page.getByLabel('Password').blur();
		await expect(main).not.toHaveAttribute('data-flat');
		await settled(page);
		expect((await layout()).lead).toBe('0');
	});

	test('adding a person opens in the sheet', async ({ page }) => {
		await page.goto('/login?add=1');
		await expect(sheet(page)).toBeVisible();
		await expect(page.getByRole('heading', { name: 'Sign in another person' })).toBeVisible();
		expect(await page.evaluate(() => document.activeElement === document.body)).toBe(true);
	});
});
