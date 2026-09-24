import { expect, test, type Page, type TestInfo } from '@playwright/test';
import {
	createRecipe,
	createUser,
	loadFixture,
	login,
	openRecipeMenu,
	setRecipesLockedByDefault,
	signOut,
	uniqueToken
} from './helpers';

/**
 * Signs whoever is in out and `username` in. The detail page replaces the
 * top bar's actions and hides the phone's bottom nav, so the sign-out menu
 * is only reachable from the overview; and the URL check waits out the
 * sign-in, or the next goto races the session cookie back to /login.
 */
async function switchTo(page: Page, testInfo: TestInfo, username: string) {
	await page.goto('/');
	await signOut(page, testInfo);
	await login(page, username);
	await expect(page).toHaveURL('/');
}

test('the household lock hides editing from other members until the author opens the recipe', async ({
	page,
	browser
}, testInfo) => {
	// One switch for the whole instance: were the desktop and mobile projects
	// both to flip it, one project's reset would open the household in the
	// middle of the other's test. The menu checked below is the same
	// component on both viewports.
	test.skip(
		testInfo.project.name.startsWith('mobile'),
		'the household lock is instance-wide; one project flips it'
	);
	// The household lock is instance-wide and the suite shares one instance,
	// so the test turns it off again whatever happened. The reset lives here
	// rather than in an afterEach hook, because Playwright runs afterEach for
	// the mobile project's skipped copy too - which would open the household
	// in the middle of the desktop run.
	try {
		const token = uniqueToken();
		const anna = `anna${token}`.slice(0, 20);
		const ben = `ben${token}`.slice(0, 20);

		await login(page);
		await expect(page).toHaveURL('/');
		await createUser(page, { username: anna, role: 'user' });
		await createUser(page, { username: ben, role: 'user' });

		// The owner flips the household lock where the household is run.
		await page.goto('/settings/users');
		const lock = page.getByRole('switch', { name: 'Only authors and admins edit recipes' });
		await lock.click();
		await expect(page.getByText('Recipes are now locked to their authors')).toBeVisible();
		await expect(lock).toBeChecked();

		await switchTo(page, testInfo, anna);
		const recipe = await createRecipe(page, {
			...loadFixture(0),
			title: `Anna's bread ${token}`
		});

		// Ben reads the recipe but may neither edit nor delete it, and the
		// colophon says who can.
		await switchTo(page, testInfo, ben);
		await page.goto(`/recipes/${recipe.slug}`);
		await expect(page.getByText(`Only ${anna} and admins can edit this recipe.`)).toBeVisible();
		await openRecipeMenu(page);
		await expect(page.getByRole('menuitem', { name: 'Copy link' })).toBeVisible();
		await expect(page.getByRole('menuitem', { name: 'Edit' })).toHaveCount(0);
		await expect(page.getByRole('menuitem', { name: 'Delete' })).toHaveCount(0);
		await page.keyboard.press('Escape');
		await expect(page.getByRole('link', { name: 'Edit', exact: true })).toHaveCount(0);

		// A bookmarked editor sends him back to the recipe.
		await page.goto(`/recipes/${recipe.slug}/edit`);
		await expect(page).toHaveURL(`/recipes/${recipe.slug}`);

		// Anna opens her recipe to everyone from the editor's last section.
		await switchTo(page, testInfo, anna);
		await page.goto(`/recipes/${recipe.slug}`);
		await expect(page.getByText('Only you and admins can edit this recipe.')).toBeVisible();
		await page.goto(`/recipes/${recipe.slug}/edit`);
		await page.getByRole('radio', { name: 'Everyone' }).click();
		await expect(
			page.getByText(
				'Everyone in the household may edit this recipe. Only you and admins can delete it.'
			)
		).toBeVisible();
		await page.getByRole('button', { name: 'Save', exact: true }).click();
		await expect(page).toHaveURL(`/recipes/${recipe.slug}`);
		await expect(page.getByText('and admins can edit this recipe.')).toHaveCount(0);

		// Ben may edit it now - but opening a recipe never opens deleting it.
		await switchTo(page, testInfo, ben);
		await page.goto(`/recipes/${recipe.slug}`);
		await expect(
			page.getByRole('heading', { level: 1, name: `Anna's bread ${token}` })
		).toBeVisible();
		await expect(page.getByText('and admins can edit this recipe.')).toHaveCount(0);
		await openRecipeMenu(page);
		await expect(page.getByRole('menuitem', { name: 'Edit' })).toBeVisible();
		await expect(page.getByRole('menuitem', { name: 'Delete' })).toHaveCount(0);
		await page.getByRole('menuitem', { name: 'Edit' }).click();
		await expect(page).toHaveURL(`/recipes/${recipe.slug}/edit`);
		// He is not the author, so he reads who opened it instead of a control.
		await expect(page.getByText(`${anna} has opened this recipe to everyone.`)).toBeVisible();
		await expect(page.getByRole('radio', { name: 'Everyone' })).toHaveCount(0);
	} finally {
		const admin = await browser.newPage();
		await login(admin);
		await expect(admin).toHaveURL('/');
		await setRecipesLockedByDefault(admin, false);
		await admin.close();
	}
});
