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
	await page.getByLabel('Name').fill(name);
	await page.getByRole('button', { name: 'Create list' }).click();
	await expect(page.getByRole('heading', { level: 1, name })).toBeVisible();
}

/** Types into the name field alone: the server still reads "2 l Milch" as amount, unit and name. */
async function addFree(page: Page, text: string) {
	await page.getByLabel('What do you need?').fill(text);
	await page.getByRole('button', { name: 'Add', exact: true }).click();
	await expect(page.getByLabel('What do you need?')).toHaveValue('');
}

test('a person without lists sees the steps, and the button starts one', async ({ page }) => {
	const person = `empty${uniqueToken()}`.slice(0, 20);
	await login(page);
	await createUser(page, { username: person, role: 'user' });
	await login(page, person);
	await page.goto('/lists');

	await expect(page.getByRole('heading', { level: 2, name: 'No shopping list yet' })).toBeVisible();
	const steps = page.getByRole('list').filter({ hasText: 'Add recipes' });
	await expect(steps.getByRole('listitem')).toHaveText([
		/Start a list/,
		/Add recipes/,
		/Fill in and share/
	]);
	await page.getByRole('button', { name: 'New list' }).click();

	// The draft: the name has the caret, "Create" waits for one, and the rest is not there yet.
	await expect(page).toHaveURL(/\/lists\/new$/);
	const name = page.getByLabel('Name');
	await expect(name).toBeFocused();
	await expect(page.getByRole('button', { name: 'Create list' })).toBeDisabled();
	await expect(page.locator('[inert] input[placeholder="What do you need?"]')).toHaveCount(1);

	// Cancel leaves nothing behind, a typed name included.
	await name.fill('Wochenmarkt');
	await page.getByRole('link', { name: 'Cancel' }).filter({ visible: true }).click();
	await expect(page).toHaveURL(/\/lists$/);
	await expect(page.getByRole('heading', { level: 2, name: 'No shopping list yet' })).toBeVisible();

	// Enter creates it, its page replaces the draft, and the caret waits for the first item.
	await page.getByRole('button', { name: 'New list' }).click();
	await page.getByLabel('Name').fill('Wochenmarkt');
	await page.getByLabel('Name').press('Enter');
	await expect(page.getByRole('heading', { level: 1, name: 'Wochenmarkt' })).toBeVisible();
	await expect(page.getByLabel('What do you need?')).toBeFocused();
	await expect(
		page.getByRole('button', { name: 'Share' }).filter({ visible: true }).first()
	).toBeVisible();
	await page.goBack();
	await expect(page).toHaveURL(/\/lists$/);
	await expect(page.getByRole('link', { name: /Wochenmarkt/ })).toBeVisible();
});

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

	// Adding it again, the dialog says it is there already.
	await page.goto(`/recipes/${recipe.slug}`);
	await page.getByRole('button', { name: 'Add to shopping list' }).click();
	await expect(
		page.getByRole('dialog').getByText('Already on this list: the amounts will be added to it.')
	).toBeVisible();
	await page.keyboard.press('Escape');
	await page.goto(page.url().replace(/\/recipes\/.*/, '/lists'));
	await page.getByRole('link', { name: new RegExp(`Woche ${token}`) }).click();
	await expect(page.getByRole('heading', { level: 1, name: `Woche ${token}` })).toBeVisible();

	// Scaled to six: 500 g mince for four becomes 750 g.
	await expect(page.getByRole('button', { name: 'Rinderhackfleisch', exact: true })).toBeVisible();
	await expect(page.getByText('750 g', { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Salz', exact: true })).toHaveCount(0);

	await addFree(page, '2 l Milch');
	await expect(page.getByRole('button', { name: 'Milch', exact: true })).toBeVisible();
	// Amount and unit in their own fields; the toast names where it was sorted.
	await page.getByLabel('Amount', { exact: true }).fill('1,5');
	await page.getByLabel('Unit', { exact: true }).fill('Becher');
	await page.getByLabel('What do you need?').fill('Joghurt');
	await page.getByLabel('What do you need?').press('Enter');
	await expect(page.getByText('Joghurt sorted under Dairy & chilled')).toBeVisible();
	await expect(department(page, 'Dairy & chilled').getByText('1 ½ Becher')).toBeVisible();
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
	// Wait for the list first, or the card still on screen answers for it.
	// The page says its counts three ways (the phone's Shop button, that
	// button's description, the desktop list card), all from the same rows.
	await expect(page.getByRole('heading', { level: 1, name: `Woche ${token}` })).toBeVisible();
	await expect(page.locator('#list-shop-counts')).toHaveText(counts);
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
	await dialog.getByRole('combobox', { name: 'Invite people' }).click();
	await dialog.getByRole('option', { name }).click();
	await dialog.getByRole('button', { name: 'Invite 1 person' }).click();
	await expect(dialog.getByText('Waiting for an answer')).toBeVisible();
	return dialog;
}

test('people are picked first and invited together; closing drops the pick', async ({ page }) => {
	const token = uniqueToken();
	const owner = `own${token}`.slice(0, 20);
	const [a, b, c] = ['a', 'b', 'c'].map((s) => `in${s}${token}`.slice(0, 20));
	await login(page);
	await createUser(page, { username: owner, role: 'user' });
	// Two people with one first name tell apart by their username.
	await createUser(page, { username: a, role: 'user', displayName: `Mila ${token}` });
	await createUser(page, { username: b, role: 'user', displayName: `Mila ${token}` });
	await createUser(page, { username: c, role: 'user', displayName: `Jonas ${token}` });
	await login(page, owner);
	await newList(page, `Grill ${token}`);

	await page.getByRole('button', { name: 'Share', exact: true }).first().click();
	const dialog = page.getByRole('dialog');
	const search = dialog.getByRole('combobox', { name: 'Invite people' });
	await search.click();
	// Suggested without typing; typing filters by name and username.
	await expect(dialog.getByRole('option', { name: `@${c}` })).toBeVisible();
	await search.fill(a);
	await expect(dialog.getByRole('option')).toHaveCount(1);
	await dialog.getByRole('option', { name: `@${a}` }).click();
	await search.fill(`jonas ${token}`);
	await dialog.getByRole('option', { name: `@${c}` }).click();
	// Backspace in the empty field takes the last pick off again.
	await search.press('Backspace');
	await expect(dialog.getByRole('button', { name: /off the selection$/ })).toHaveCount(1);
	await search.fill(`jonas ${token}`);
	await dialog.getByRole('option', { name: `@${c}` }).click();

	// Picking sends nothing yet.
	await expect(dialog.getByText('Waiting for an answer')).toHaveCount(0);
	await dialog.getByRole('button', { name: 'Invite 2 people' }).click();
	await expect(dialog.getByRole('status')).toHaveText(
		`Invited: Mila ${token} and Jonas ${token}. Notifications are on their way.`
	);
	await expect(dialog.getByText('Waiting for an answer')).toHaveCount(2);
	await expect(dialog.getByRole('button', { name: /^Invite \d/ })).toHaveCount(0);

	// The invited ones are no longer suggested; picked and closed, nobody is invited.
	// Other tests' accounts share the instance, so this looks at these three only.
	await search.click();
	await expect(dialog.getByRole('option', { name: `@${a}` })).toHaveCount(0);
	await expect(dialog.getByRole('option', { name: `@${c}` })).toHaveCount(0);
	await dialog.getByRole('option', { name: `@${b}` }).click();
	await page.keyboard.press('Escape');
	await page.keyboard.press('Escape');
	await expect(dialog).toHaveCount(0);
	await page.getByRole('button', { name: 'Share', exact: true }).first().click();
	await expect(page.getByRole('dialog').getByText('Waiting for an answer')).toHaveCount(2);
	await expect(page.getByRole('dialog').getByRole('button', { name: /^Remove / })).toHaveCount(2);
	// Reopened, the pick from before is gone.
	await expect(
		page.getByRole('dialog').getByRole('button', { name: /off the selection$/ })
	).toHaveCount(0);
});

test('a failed invitation keeps its pick to try again', async ({ page }) => {
	const token = uniqueToken();
	const owner = `own${token}`.slice(0, 20);
	const [a, b] = ['a', 'b'].map((s) => `fl${s}${token}`.slice(0, 20));
	await login(page);
	await createUser(page, { username: owner, role: 'user' });
	await createUser(page, { username: a, role: 'user', displayName: `Ada ${token}` });
	const bea = await createUser(page, { username: b, role: 'user', displayName: `Bea ${token}` });
	await login(page, owner);
	await newList(page, `Fest ${token}`);

	// Bea's invitation fails once; Ada's goes through.
	let failOnce = true;
	await page.route('**/api/v1/shopping-lists/*/invitations', async (route) => {
		const body = route.request().postDataJSON() as { userId: string };
		if (failOnce && body.userId === bea.id) {
			failOnce = false;
			await route.fulfill({ status: 500, body: '{}' });
		} else await route.continue();
	});

	await page.getByRole('button', { name: 'Share', exact: true }).first().click();
	const dialog = page.getByRole('dialog');
	const search = dialog.getByRole('combobox', { name: 'Invite people' });
	await search.click();
	await search.fill(a);
	await dialog.getByRole('option', { name: `@${a}` }).click();
	await search.fill(b);
	await dialog.getByRole('option', { name: `@${b}` }).click();
	await dialog.getByRole('button', { name: 'Invite 2 people' }).click();

	await expect(dialog.getByRole('status')).toHaveText(
		`Could not invite Bea ${token}. They are still picked - try again.`
	);
	await expect(dialog.getByText('Waiting for an answer')).toHaveCount(1);
	await expect(dialog.getByText('Not invited')).toHaveCount(1);
	await expect(
		dialog.getByRole('button', { name: `Take Bea ${token} off the selection` })
	).toBeVisible();

	await dialog.getByRole('button', { name: 'Invite 1 person' }).click();
	await expect(dialog.getByRole('status')).toHaveText(
		`Invited: Bea ${token}. A notification is on its way.`
	);
	await expect(dialog.getByText('Waiting for an answer')).toHaveCount(2);
	await expect(dialog.getByRole('button', { name: /off the selection$/ })).toHaveCount(0);
});

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
		await expect(lists.getByText('1 invitation', { exact: true })).toBeAttached();
		await lists.getByRole('button', { name: 'Accept' }).click();
		await expect(lists.getByRole('heading', { level: 1, name: `Grill ${token}` })).toBeVisible();
		// The member sees who is on the list, on any screen: the faces open it.
		await lists.getByRole('button', { name: '2 people on this list' }).click();
		const people = lists.getByRole('dialog', { name: 'Shared with' });
		await expect(people.getByText('You · can add and tick')).toBeVisible();
		await expect(people.getByText(owner, { exact: true })).toBeVisible();
		await people.getByRole('button', { name: 'Done' }).last().click();
		await lists.getByRole('button', { name: 'Check off Kohle' }).click();
		await expect(lists.getByRole('heading', { name: 'Done · 1' })).toBeVisible();

		// In shopping mode she sees whose ticks these are: hers, beside the owner's face.
		await lists.goto(`${listUrl}/shop`);
		await expect(lists.getByRole('img', { name: `Checking off as ${mila}` })).toBeVisible();
		await expect(lists.getByText(`Shared with ${owner}`, { exact: true })).toBeAttached();
		await lists.goto(listUrl);

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
	for (const name of [`Nein ${token}`, `Ja ${token}`, `Weg ${token}`]) {
		await newList(page, name);
		const dialog = await shareWith(page, mila);
		await dialog.getByRole('button', { name: 'Done' }).filter({ visible: true }).first().click();
	}
	const gone = page.url().split('/').pop();

	const guest = await signedInAs(browser, mila);
	try {
		const lists = guest.page;
		await lists.goto('/lists');
		await expect(lists.getByText('3 invitations')).toBeAttached();

		// Withdrawn while the page is open: answering it drops the card.
		const origin = new URL(page.url()).origin;
		const withdrawn = await page.request.delete(`/api/v1/shopping-lists/${gone}`, {
			headers: { Origin: origin }
		});
		expect(withdrawn.ok()).toBe(true);
		// The innermost element that holds both the text and the button is the card.
		const accept = lists.getByRole('button', { name: 'Accept' });
		await lists
			.locator('div', { hasText: `invites you to “Weg ${token}”` })
			.filter({ has: accept })
			.last()
			.getByRole('button', { name: 'Accept' })
			.click();
		await expect(lists.getByText('This list is no longer there for you')).toBeVisible();
		await expect(lists.getByText(`invites you to “Weg ${token}”`)).toHaveCount(0);
		await expect(lists.getByText('2 invitations')).toBeAttached();

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
	await createUser(page, { username: mila, role: 'user', displayName: 'Mila Majoran' });
	await login(page, owner);

	await newList(page, `Live ${token}`);
	await addFree(page, 'Kohle');
	const dialog = await shareWith(page, `@${mila}`);
	await dialog.getByRole('button', { name: 'Done' }).filter({ visible: true }).first().click();

	const guest = await signedInAs(browser, mila);
	try {
		const b = guest.page;
		await b.goto('/lists');
		await b.getByRole('button', { name: 'Accept' }).click();
		await expect(b.getByRole('heading', { level: 1, name: `Live ${token}` })).toBeVisible();

		// B ticks; A, who did not reload, sees it: ticked in its place for a moment, then under Done.
		await b.getByRole('button', { name: 'Check off Kohle' }).click();
		await expect(page.getByRole('button', { name: 'Check off Kohle' })).toHaveAttribute(
			'aria-pressed',
			'true',
			{ timeout: 3000 }
		);
		await expect(page.getByRole('heading', { name: /^Done/ })).toHaveCount(0);
		await expect(page.getByRole('heading', { name: 'Done · 1' })).toBeVisible({ timeout: 3000 });
		// A shared list's done row shows who ticked it.
		await expect(page.getByRole('img', { name: 'Ticked by Mila Majoran' })).toBeVisible();

		// B types an item in; A sees whose it is: B's mark, then her first name.
		await b.getByLabel('What do you need?').fill('Salz');
		await b.getByRole('button', { name: 'Add', exact: true }).click();
		const salz = page.getByRole('main').locator('li').filter({ hasText: 'Salz' });
		const faces = salz.locator('.rounded-full[aria-hidden="true"]');
		await expect(salz.getByText('by Mila', { exact: true })).toBeVisible({ timeout: 3000 });
		await expect(faces).toHaveCount(1);
		// A ticks it: the row keeps one face, the ticker's, and names the adder in words.
		await page.getByRole('button', { name: 'Check off Salz' }).click();
		await expect(salz.getByRole('img', { name: `Ticked by ${owner}` })).toBeVisible();
		await expect(salz.getByText('by Mila', { exact: true })).toBeVisible();
		await expect(faces).toHaveCount(1);

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

test('shopping mode ticks and collapses done items', async ({ page }, testInfo) => {
	const token = uniqueToken();
	const person = `mode${token}`.slice(0, 20);
	await login(page);
	await createUser(page, { username: person, role: 'user' });
	await login(page, person);
	await newList(page, `Laden ${token}`);
	await addFree(page, 'Kohle');
	await addFree(page, 'Senf');

	const shop = page.getByRole('link', { name: 'Shop', exact: true }).filter({ visible: true });
	// On a phone the button carries the counts, read as its description.
	if (testInfo.project.name.startsWith('mobile'))
		await expect(shop).toHaveAccessibleDescription('2 open · 0 done');
	await shop.click();
	await expect(page).toHaveURL(/\/lists\/[^/]+\/shop$/);
	await expect(page.getByRole('main', { name: 'Shopping mode' })).toBeVisible();
	// The head names the account the ticks go to; a list of one shares with no one.
	await expect(page.getByRole('img', { name: `Checking off as ${person}` })).toBeVisible();
	await expect(page.getByText(/^Shared with/)).toHaveCount(0);
	// Nothing to type in the store: the list is kept before.
	await expect(page.getByRole('textbox')).toHaveCount(0);

	// A tap anywhere on the row ticks, not only on the circle; the row leaves the open groups.
	await page
		.getByRole('listitem')
		.filter({ has: page.getByText('Kohle', { exact: true }) })
		.click();
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
