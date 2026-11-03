import { expect, test, type Page } from '@playwright/test';
import { createRecipe, createUser, loadFixture, login, uniqueToken } from './helpers';

// A list is its creator's alone, so each test signs in as a person of its
// own: the desktop and mobile projects run at the same time on one instance.
// German data on purpose - the department sorting reads German and English
// names, and "Zwiebel" and "Kapern" are what it has to place.

/** The department block headed `label` on the list page. */
const department = (page: Page, label: string) =>
	page
		.locator('section')
		.filter({ has: page.getByRole('heading', { name: new RegExp(`^${label}`) }) });

async function newList(page: Page, name: string) {
	await page.goto('/lists');
	await page.getByRole('button', { name: 'New list' }).filter({ visible: true }).first().click();
	await page.getByRole('dialog').getByLabel('Name').fill(name);
	await page.getByRole('button', { name: 'Create list' }).click();
	await expect(page.getByRole('heading', { level: 1, name })).toBeVisible();
}

async function addFree(page: Page, text: string) {
	await page.getByLabel('Add something, e.g. milk').fill(text);
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	await expect(page.getByLabel('Add something, e.g. milk')).toHaveValue('');
}

test('a shopping list fills from a recipe and by hand', async ({ page }) => {
	const token = uniqueToken();
	const person = `shop${token}`.slice(0, 20);
	await login(page);
	await createUser(page, { username: person, role: 'user' });
	await login(page, person);
	const recipe = await createRecipe(page, { ...loadFixture(0, 'de'), title: `Klopse ${token}` });

	await newList(page, `Woche ${token}`);
	await expect(page.getByText('Nothing on the list yet')).toBeVisible();

	// From the recipe: six servings, without the salt kept at home.
	await page.goto(`/recipes/${recipe.slug}`);
	await page.getByRole('button', { name: 'Add to shopping list' }).click();
	const dialog = page.getByRole('dialog');
	await expect(dialog.getByText('Ingredients · 14 of 14')).toBeVisible();
	const salt = dialog.getByRole('checkbox', { name: 'Salz', exact: true });
	await salt.first().click();
	await salt.last().click();
	const more = dialog.getByRole('button', { name: 'More servings' });
	await more.click();
	await more.click();
	await dialog.getByRole('button', { name: 'Add 12 ingredients' }).click();
	await page.getByRole('button', { name: 'Open list' }).click();
	await expect(page.getByRole('heading', { level: 1, name: `Woche ${token}` })).toBeVisible();

	// Scaled to six: 500 g mince for four becomes 750 g.
	await expect(page.getByRole('button', { name: 'Rinderhackfleisch', exact: true })).toBeVisible();
	await expect(page.getByText('750 g', { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Salz', exact: true })).toHaveCount(0);

	await addFree(page, '2 l Milch');
	await expect(page.getByRole('button', { name: 'Milch', exact: true })).toBeVisible();
	// "Eier" joins the recipe's "Ei": one row with two amounts.
	await addFree(page, '2 Eier');
	await expect(page.getByRole('button', { name: 'Check off Ei', exact: true })).toHaveCount(1);

	// The recipe view: the recipe's block at its servings, the typed-in one apart.
	await page.getByRole('radio', { name: 'Recipe' }).click();
	await expect(page.getByRole('heading', { level: 2, name: `Klopse ${token}` })).toBeVisible();
	await expect(page.getByText('6 servings', { exact: true }).first()).toBeVisible();
	await expect(page.getByRole('heading', { level: 2, name: 'Without a recipe' })).toBeVisible();
	await page.getByRole('radio', { name: 'Department' }).click();
	await expect(page.getByRole('heading', { level: 2, name: 'Without a recipe' })).toHaveCount(0);

	// Ticked, it moves to Done; "Remove done" takes it off.
	await page.getByRole('button', { name: 'Check off Zwiebel' }).click();
	await expect(page.getByRole('heading', { name: 'Done · 1' })).toBeVisible();
	await page.getByRole('button', { name: 'Remove done' }).click();
	await expect(page.getByRole('heading', { name: /^Done/ })).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Check off Zwiebel' })).toHaveCount(0);

	// Moved to the chilled shelf, capers stay there on every list.
	await page.getByRole('button', { name: 'Kapern', exact: true }).click();
	const sheet = page.getByRole('dialog', { name: 'Kapern' });
	await sheet.getByRole('radio', { name: 'Dairy & chilled' }).click();
	// The phone sheet's handle is labeled "Done" too; the footer button is the one with the word.
	await sheet
		.getByRole('button', { name: 'Done', exact: true })
		.filter({ hasText: 'Done' })
		.click();
	await expect(
		department(page, 'Dairy & chilled').getByRole('button', { name: 'Kapern', exact: true })
	).toBeVisible();

	await newList(page, `Markt ${token}`);
	await addFree(page, 'Kapern');
	await expect(
		department(page, 'Dairy & chilled').getByRole('button', { name: 'Kapern', exact: true })
	).toBeVisible();

	// Deleted, the list leaves the overview; the other stays.
	await page.getByRole('button', { name: 'More actions' }).click();
	await page.getByRole('menuitem', { name: 'Delete list' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Delete', exact: true }).click();
	await expect(page).toHaveURL('/lists');
	await expect(page.getByRole('heading', { level: 3, name: `Woche ${token}` })).toBeVisible();
	await expect(page.getByRole('heading', { level: 3, name: `Markt ${token}` })).toHaveCount(0);

	// The card and the list's header count the same rows ("Ei" and "Eier" are one).
	const card = page.getByRole('link', { name: new RegExp(`Woche ${token}`) });
	const counts = (await card.getByText(/^\d+ open · \d+ done$/).textContent())!.trim();
	await card.click();
	await expect(page.getByText(counts, { exact: true })).toBeVisible();
});
