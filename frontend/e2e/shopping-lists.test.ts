import { expect, test, type Browser, type Page } from '@playwright/test';
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

/** A second browser, signed in as `name`. */
async function signedInAs(browser: Browser, name: string) {
	const context = await browser.newContext();
	const page = await context.newPage();
	await login(page, name);
	return { context, page };
}

/** Invites `name` from the open share dialog of the list page. */
async function shareWith(page: Page, name: string) {
	await page.getByRole('button', { name: 'Share', exact: true }).first().click();
	const dialog = page.getByRole('dialog');
	await dialog.getByRole('button', { name: 'Invite someone' }).click();
	await page.getByRole('option', { name }).click();
	await expect(dialog.getByText('Waiting for an answer')).toBeVisible();
	return dialog;
}

test('a shared list is ticked by both, until the owner removes the member', async ({
	page,
	browser
}, testInfo) => {
	// Two people at once; the share flow is the same component on both viewports.
	test.skip(testInfo.project.name.startsWith('mobile'), 'two browsers; runs once, on desktop');
	const token = uniqueToken();
	const owner = `own${token}`.slice(0, 20);
	const mila = `mil${token}`.slice(0, 20);
	await login(page);
	await createUser(page, { username: owner, role: 'user' });
	await createUser(page, { username: mila, role: 'user' });
	await login(page, owner);

	await newList(page, `Grill ${token}`);
	await addFree(page, 'Kohle');
	await addFree(page, 'Brot');
	const listUrl = page.url();
	const dialog = await shareWith(page, mila);
	await dialog.getByRole('button', { name: 'Done' }).filter({ visible: true }).first().click();

	const guest = await signedInAs(browser, mila);
	try {
		const lists = guest.page;
		await lists.goto('/lists');
		await expect(lists.getByText(`${owner} invites you to “Grill ${token}”`)).toBeVisible();
		await expect(lists.getByText('1 invitations')).toBeAttached();
		await lists.getByRole('button', { name: 'Accept' }).click();
		await expect(lists.getByRole('heading', { level: 1, name: `Grill ${token}` })).toBeVisible();
		await lists.getByRole('button', { name: 'Check off Kohle' }).click();
		await expect(lists.getByRole('heading', { name: 'Done · 1' })).toBeVisible();

		// The owner sees the tick, and who changed the list.
		await page.goto(listUrl);
		await expect(page.getByRole('heading', { name: 'Done · 1' })).toBeVisible();
		await page.goto('/lists');
		await expect(page.getByText(new RegExp(`Changed by ${mila}`))).toBeVisible();

		// Removed, her next change finds the list gone.
		await page.goto(listUrl);
		await page.getByRole('button', { name: 'Share', exact: true }).first().click();
		await page
			.getByRole('dialog')
			.getByRole('button', { name: `Remove ${mila}` })
			.click();
		await expect(page.getByRole('dialog').getByText(mila)).toHaveCount(0);

		await lists.getByRole('button', { name: 'Check off Brot' }).click();
		await expect(lists.getByText('This list is no longer there for you')).toBeVisible();
		await expect(lists).toHaveURL('/lists');
	} finally {
		await guest.context.close();
	}
});

test('an invitation is declined, and a member leaves', async ({ page, browser }, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'two browsers; runs once, on desktop');
	const token = uniqueToken();
	const owner = `own${token}`.slice(0, 20);
	const mila = `mil${token}`.slice(0, 20);
	await login(page);
	await createUser(page, { username: owner, role: 'user' });
	await createUser(page, { username: mila, role: 'user' });
	await login(page, owner);
	for (const name of [`Nein ${token}`, `Ja ${token}`]) {
		await newList(page, name);
		const dialog = await shareWith(page, mila);
		await dialog.getByRole('button', { name: 'Done' }).filter({ visible: true }).first().click();
	}

	const guest = await signedInAs(browser, mila);
	try {
		const lists = guest.page;
		await lists.goto('/lists');
		await expect(lists.getByText(`invites you to “Nein ${token}”`)).toBeVisible();
		await lists
			.locator('div', { hasText: `invites you to “Nein ${token}”` })
			.getByRole('button', { name: 'Decline' })
			.last()
			.click();
		await expect(lists.getByText(`invites you to “Nein ${token}”`)).toHaveCount(0);

		await lists
			.locator('div', { hasText: `invites you to “Ja ${token}”` })
			.getByRole('button', { name: 'Accept' })
			.last()
			.click();
		await expect(lists.getByRole('heading', { level: 1, name: `Ja ${token}` })).toBeVisible();

		// A member may leave but not share or delete.
		await expect(lists.getByRole('button', { name: 'Share', exact: true })).toHaveCount(0);
		await lists.getByRole('button', { name: 'More actions' }).click();
		await expect(lists.getByRole('menuitem', { name: 'Delete list' })).toHaveCount(0);
		await lists.getByRole('menuitem', { name: 'Leave list' }).click();
		await lists.getByRole('dialog').getByRole('button', { name: 'Leave list' }).click();
		await expect(lists).toHaveURL('/lists');
		await expect(lists.getByRole('heading', { level: 3, name: `Ja ${token}` })).toHaveCount(0);
		await expect(lists.getByRole('heading', { level: 3, name: `Nein ${token}` })).toHaveCount(0);
	} finally {
		await guest.context.close();
	}
});

test('a change by one member appears for the other without reload', async ({
	page,
	browser
}, testInfo) => {
	test.skip(testInfo.project.name.startsWith('mobile'), 'two browsers; runs once, on desktop');
	const token = uniqueToken();
	const owner = `own${token}`.slice(0, 20);
	const mila = `mil${token}`.slice(0, 20);
	await login(page);
	await createUser(page, { username: owner, role: 'user' });
	await createUser(page, { username: mila, role: 'user' });
	await login(page, owner);

	await newList(page, `Live ${token}`);
	await addFree(page, 'Kohle');
	const dialog = await shareWith(page, mila);
	await dialog.getByRole('button', { name: 'Done' }).filter({ visible: true }).first().click();

	const guest = await signedInAs(browser, mila);
	try {
		const b = guest.page;
		await b.goto('/lists');
		await b.getByRole('button', { name: 'Accept' }).click();
		await expect(b.getByRole('heading', { level: 1, name: `Live ${token}` })).toBeVisible();

		// B ticks; A, who did not reload, sees it.
		await b.getByRole('button', { name: 'Check off Kohle' }).click();
		await expect(page.getByRole('heading', { name: 'Done · 1' })).toBeVisible({ timeout: 3000 });

		// A renames; B's heading follows.
		await page.getByRole('button', { name: 'More actions' }).click();
		await page.getByRole('menuitem', { name: 'Rename' }).click();
		await page.getByRole('dialog').getByLabel('Name').fill(`Neu ${token}`);
		await page.getByRole('dialog').getByRole('button', { name: 'Save' }).click();
		await expect(b.getByRole('heading', { level: 1, name: `Neu ${token}` })).toBeVisible({
			timeout: 3000
		});
	} finally {
		await guest.context.close();
	}
});

test('shopping mode ticks and collapses done items', async ({ page }) => {
	const token = uniqueToken();
	const person = `mode${token}`.slice(0, 20);
	await login(page);
	await createUser(page, { username: person, role: 'user' });
	await login(page, person);
	await newList(page, `Laden ${token}`);
	await addFree(page, 'Kohle');
	await addFree(page, 'Senf');

	await page.getByRole('link', { name: 'Shop', exact: true }).filter({ visible: true }).click();
	await expect(page).toHaveURL(/\/lists\/[^/]+\/shop$/);
	await expect(page.getByRole('main', { name: 'Shopping mode' })).toBeVisible();

	// A tap anywhere on the row ticks, not only on the circle; the row leaves the open groups.
	await page.getByRole('listitem').filter({ hasText: 'Kohle' }).click();
	const done = page.getByRole('button', { name: '1 done' });
	await expect(done).toHaveAttribute('aria-expanded', 'false');
	await expect(page.getByText('Kohle', { exact: true })).toHaveCount(0);
	await done.click();
	await expect(done).toHaveAttribute('aria-expanded', 'true');
	await expect(page.getByRole('button', { name: 'Check off Kohle' })).toHaveAttribute(
		'aria-pressed',
		'true'
	);

	await page.getByRole('link', { name: 'Leave shopping mode' }).click();
	await expect(page.getByRole('heading', { level: 1, name: `Laden ${token}` })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Done · 1' })).toBeVisible();
});
