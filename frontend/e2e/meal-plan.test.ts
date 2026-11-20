import { expect, test, type Page } from '@playwright/test';
import { createUser, login, uniqueToken } from './helpers';

const phone = (page: Page) => (page.viewportSize()?.width ?? 1280) < 768;

/** The place's link: the top bar's segment on a desktop, the bottom nav's basket on a phone. */
const placeLink = (page: Page) =>
	phone(page)
		? page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: 'Shopping & week' })
		: page
				.getByRole('banner')
				.getByRole('navigation')
				.getByRole('link', { name: 'Shopping & week' });

/** A person of the test's own: the projects run at once on one instance. */
async function loginFresh(page: Page) {
	const username = `plan${uniqueToken()}`.slice(0, 20);
	await login(page);
	await createUser(page, { username, role: 'user' });
	await page.context().clearCookies();
	await login(page, username);
}

const side = (page: Page, name: 'Lists' | 'Week') =>
	page.getByRole('radiogroup', { name: 'Lists or week' }).getByRole('radio', { name });

test('place switch remembers the side', async ({ page }) => {
	await loginFresh(page);
	// A first visit opens the lists.
	await expect(placeLink(page)).toHaveAttribute('href', '/lists');
	await placeLink(page).click();
	await expect(page).toHaveURL(/\/lists$/);
	await expect(page.getByRole('heading', { level: 1, name: 'Shopping & week' })).toBeVisible();
	await expect(side(page, 'Lists')).toBeChecked();

	await side(page, 'Week').click();
	await expect(page).toHaveURL(/\/week$/);
	await expect(page.getByRole('heading', { level: 1, name: 'Shopping & week' })).toBeVisible();
	await expect(side(page, 'Week')).toBeChecked();
	await expect(page.getByRole('heading', { level: 2, name: /Week \d+ · |\d/ })).toBeVisible();
	if (!phone(page)) {
		// The week's own action replaces New list.
		const bar = page.getByRole('banner');
		await expect(bar.getByRole('button', { name: 'Plan a meal' })).toBeVisible();
		await expect(bar.getByRole('button', { name: 'New list' })).toHaveCount(0);
	}

	// The place reopens on the week, also after a reload.
	await page.goto('/');
	await expect(placeLink(page)).toHaveAttribute('href', '/week');
	await placeLink(page).click();
	await expect(page).toHaveURL(/\/week$/);
	await expect(side(page, 'Week')).toBeChecked();

	await side(page, 'Lists').click();
	await expect(page).toHaveURL(/\/lists$/);
	await expect(placeLink(page)).toHaveAttribute('href', '/lists');
});

test('the pager moves by weeks and Today comes back', async ({ page }) => {
	await page.clock.setFixedTime(new Date('2026-11-20T10:00:00'));
	await loginFresh(page);
	await page.goto('/week');
	const label = page.getByRole('heading', { level: 2 }).first();
	await expect(label).toContainText(/November 16–22|Nov 16–22/);
	await page.getByRole('button', { name: 'Next week' }).click();
	await expect(page).toHaveURL(/\/week\?from=2026-11-23$/);
	await expect(label).toContainText(/November 23–29|Nov 23–29/);
	await page.getByRole('button', { name: 'Today' }).click();
	await expect(page).toHaveURL(/\/week$/);
	await page.getByRole('button', { name: 'Previous week' }).click();
	await expect(page).toHaveURL(/\/week\?from=2026-11-09$/);
	await expect(
		page.getByRole('heading', { level: 2, name: 'Nothing planned this week' })
	).toBeVisible();
});

// The pick lands on another page; the keyboard stays in the switch.
test('an arrow key in the switch keeps the focus on the side it picked', async ({
	page,
	isMobile
}) => {
	test.skip(isMobile, 'arrow keys are a keyboard path');
	await loginFresh(page);
	await page.goto('/lists');
	await side(page, 'Lists').focus();
	await page.keyboard.press('ArrowRight');
	await expect(page).toHaveURL(/\/week$/);
	await expect(side(page, 'Week')).toBeChecked();
	await expect(side(page, 'Week')).toBeFocused();
	await page.keyboard.press('ArrowLeft');
	await expect(page).toHaveURL(/\/lists$/);
	await expect(side(page, 'Lists')).toBeFocused();
});

test('the picker switches to a shared plan and back to the own one', async ({ page }) => {
	await loginFresh(page);
	const name = `Familie ${uniqueToken()}`;
	const made = await page.request.post('/api/v1/meal-plans', { data: { name } });
	expect(made.ok()).toBe(true);
	const shared = (await made.json()) as { id: string };
	const overview = (await (await page.request.get('/api/v1/meal-plans')).json()) as {
		plans: { id: string; personal: boolean }[];
	};
	const own = overview.plans.find((p) => p.personal)!.id;
	await page.goto('/week');
	const picker = page.getByRole('button', { name: 'Plan', exact: true });
	await expect(picker).toHaveText('My plan');

	await picker.click();
	await page.getByRole('option', { name }).click();
	await expect(page).toHaveURL(new RegExp(`/week\\?plan=${shared.id}$`));
	await expect(picker).toHaveText(name);

	await picker.click();
	await page.getByRole('option', { name: 'My plan' }).click();
	await expect(page).toHaveURL(new RegExp(`/week\\?plan=${own}$`));
	await expect(picker).toHaveText('My plan');
	// And the week reopens on the plan picked last.
	await page.goto('/week');
	await expect(picker).toHaveText('My plan');
});
