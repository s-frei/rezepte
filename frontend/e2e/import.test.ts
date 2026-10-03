import { expect, test, type Page } from '@playwright/test';
// Type-only, like helpers.ts: the stubs below speak the app's own contract.
import type { RecipeDraft } from '../src/lib/api/drafts';
import { createRecipe, loadFixture, login, openImport, uniqueToken } from './helpers';

// Importing a recipe from a link or pasted text into an unsaved draft.
// Pasted text runs against the real service. A link cannot: the service
// refuses every address on its own network, the test server's included, and
// there is deliberately no switch - so link imports stub our own API answer.

test.beforeEach(async ({ page }) => {
	await login(page);
	await expect(page).toHaveURL('/');
});

function dialog(page: Page) {
	return page.getByRole('dialog', { name: 'Import recipe' });
}

function field(page: Page) {
	return dialog(page).getByLabel('Link or recipe text');
}

async function submit(page: Page) {
	await dialog(page).getByRole('button', { name: 'Import', exact: true }).click();
}

const linkedSoup: RecipeDraft = {
	recipe: {
		title: 'Linked soup',
		description: '',
		servings: 2,
		prepMinutes: null,
		cookMinutes: 30,
		sourceUrl: 'https://example.com/soup',
		sourceName: 'Example',
		tags: [],
		ingredientGroups: [
			{ name: null, ingredients: [{ quantity: 1, unit: 'l', name: 'broth', note: null }] }
		],
		steps: [{ text: 'Boil.', references: [] }]
	},
	review: [],
	suggestedTags: ['Soup', 'Quick'],
	photo: null,
	duplicate: {
		id: 'x',
		slug: 'linked-soup',
		title: 'Linked soup',
		createdBy: { id: 'u', username: 'mila', displayName: 'Mila', color: 'sage', avatarId: null }
	},
	truncated: false
};

test('pasted text becomes a recipe with a checked ingredient', async ({ page }) => {
	const title = `Imported ${uniqueToken()}`;
	await openImport(page);
	await field(page).fill(
		`${title}\nIngredients\n400 g flour\nsalt to taste\nMethod\nMix everything.`
	);
	await expect(dialog(page).getByText('Text', { exact: true })).toBeVisible();
	await submit(page);

	await expect(page).toHaveURL('/recipes/new');
	await expect(page.getByRole('textbox', { name: 'Title', exact: true })).toHaveValue(title);
	// "salt to taste" has no amount the parser trusts, so it asks.
	await expect(page.getByText('1 ingredient to check')).toBeVisible();
	await page.getByRole('button', { name: 'Looks right' }).click();
	await expect(page.getByText('1 ingredient to check')).toHaveCount(0);

	await page.getByRole('button', { name: 'Save', exact: true }).click();
	await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
	await expect(page.getByText('Mix everything.')).toBeVisible();
});

test('an import started on the new-recipe page fills its editor', async ({ page }) => {
	await page.goto('/recipes/new');
	const titleField = page.getByRole('textbox', { name: 'Title', exact: true });
	await expect(titleField).toHaveValue('');
	await page.keyboard.press('Control+k');
	await page
		.getByRole('dialog', { name: 'Command palette' })
		.getByRole('option', { name: 'Import recipe' })
		.click();
	const title = `Palette ${uniqueToken()}`;
	await field(page).fill(`${title}\nIngredients\n2 eggs\nMethod\nFry.`);
	await submit(page);

	await expect(dialog(page)).toHaveCount(0);
	await expect(page).toHaveURL('/recipes/new');
	await expect(titleField).toHaveValue(title);
	// The editor took the draft in place; no navigation, so no leave guard.
	await expect(page.getByRole('dialog')).toHaveCount(0);
});

/** Imports pasted text through the palette, from whatever page is open. */
async function importFromPalette(page: Page, title: string) {
	await page.keyboard.press('Control+k');
	await page
		.getByRole('dialog', { name: 'Command palette' })
		.getByRole('option', { name: 'Import recipe' })
		.click();
	await field(page).fill(`${title}\nIngredients\n2 eggs\nMethod\nFry.`);
	await submit(page);
}

// An editor with unsaved changes asks before an import replaces it - on its
// own page as on /recipes/new - and the import waits for the answer.
for (const where of ['another recipe', 'the new-recipe page'] as const) {
	async function dirtyEditor(page: Page) {
		const titleField = page.getByRole('textbox', { name: 'Title', exact: true });
		if (where === 'another recipe') {
			const fixture = loadFixture(3);
			fixture.title = `Kept ${uniqueToken()}`;
			const recipe = await createRecipe(page, fixture);
			await page.goto(`/recipes/${recipe.slug}/edit`);
		} else {
			await page.goto('/recipes/new');
		}
		await expect(titleField).toBeVisible();
		const typed = `Typed ${uniqueToken()}`;
		await titleField.fill(typed);
		return { titleField, typed };
	}

	const discard = (page: Page) => page.getByRole('dialog', { name: 'Discard changes?' });

	test(`on ${where} with unsaved changes, Discard opens the import`, async ({ page }) => {
		const { titleField } = await dirtyEditor(page);
		const title = `Imported ${uniqueToken()}`;
		await importFromPalette(page, title);
		await discard(page).getByRole('button', { name: 'Discard' }).click();
		await expect(page).toHaveURL('/recipes/new');
		await expect(titleField).toHaveValue(title);
	});

	test(`on ${where} with unsaved changes, Keep editing drops the import`, async ({ page }) => {
		const { titleField, typed } = await dirtyEditor(page);
		const url = page.url();
		await importFromPalette(page, `Imported ${uniqueToken()}`);
		await discard(page).getByRole('button', { name: 'Cancel' }).click();
		await expect(discard(page)).toHaveCount(0);
		await expect(page).toHaveURL(url);
		await expect(titleField).toHaveValue(typed);

		// Leave for real, then start a new recipe: the import is gone.
		await page.getByRole('button', { name: 'Cancel', exact: true }).first().click();
		await discard(page).getByRole('button', { name: 'Discard' }).click();
		await expect(page).not.toHaveURL(url);
		await page.keyboard.press('Control+k');
		await page
			.getByRole('dialog', { name: 'Command palette' })
			.getByRole('option', { name: 'New recipe' })
			.click();
		await expect(page).toHaveURL('/recipes/new');
		await expect(titleField).toHaveValue('');
	});
}

test('a link already in the collection says so and can be imported anyway', async ({ page }) => {
	await page.route('**/api/v1/recipe-drafts', (route) => route.fulfill({ json: linkedSoup }));
	await openImport(page);
	await field(page).fill('https://example.com/soup');
	await expect(dialog(page).getByText('to example.com')).toBeVisible();
	await submit(page);

	await expect(dialog(page).getByText('This recipe is already here:')).toBeVisible();
	await expect(dialog(page).getByRole('link', { name: 'View' })).toHaveAttribute(
		'href',
		'/recipes/linked-soup'
	);
	// Back returns to the field with the link still in it.
	await dialog(page).getByRole('button', { name: 'Back', exact: true }).click();
	await expect(field(page)).toHaveValue('https://example.com/soup');
	await expect(field(page)).toBeFocused();
	await submit(page);
	await dialog(page).getByRole('button', { name: 'Import anyway' }).click();

	await expect(page).toHaveURL('/recipes/new');
	await expect(page.getByRole('textbox', { name: 'Title', exact: true })).toHaveValue(
		'Linked soup'
	);
	// New keywords are offered, never set: each one waits for a yes.
	await page.getByRole('button', { name: '2 suggestions' }).click();
	await page.getByRole('button', { name: 'Add Soup as a tag' }).click();
	await expect(page.getByRole('button', { name: '1 suggestion' })).toBeVisible();
	await expect(page.getByText('Soup', { exact: true })).toBeVisible();
});

test('the dialog reopened while it fades out still imports', async ({ page, isMobile }) => {
	test.skip(isMobile, 'the top bar button reopens it within the fade-out');
	const trigger = page.getByRole('banner').getByRole('button', { name: 'Import', exact: true });
	await openImport(page);
	await field(page).fill('kept');
	// Close and reopen in one go, before the dialog has unmounted its form.
	await page.keyboard.press('Escape');
	await trigger.dispatchEvent('click');
	// The same form came back, not a fresh one.
	await expect(field(page)).toHaveValue('kept');
	const title = `Reopened ${uniqueToken()}`;
	await field(page).fill(`${title}\nIngredients\n400 g flour\nMethod\nMix.`);
	await submit(page);
	await expect(page).toHaveURL('/recipes/new');
	await expect(page.getByRole('textbox', { name: 'Title', exact: true })).toHaveValue(title);
});

test('closing the dialog while it reads drops the draft', async ({ page }) => {
	let release = () => {};
	const released = new Promise<void>((resolve) => (release = resolve));
	let answered = false;
	await page.route('**/api/v1/recipe-drafts', async (route) => {
		await released;
		await route.fulfill({ json: { ...linkedSoup, duplicate: null } }).catch(() => {});
		answered = true;
	});
	await openImport(page);
	await field(page).fill('https://example.com/soup');
	await submit(page);
	await expect(dialog(page).getByText('Reading example.com')).toBeVisible();
	await page.keyboard.press('Escape');
	// The answer arrives while the dialog is still fading out.
	release();
	await expect.poll(() => answered).toBe(true);
	await expect(dialog(page)).toHaveCount(0);
	await expect(page).toHaveURL('/');

	await page.keyboard.press('Control+k');
	await page
		.getByRole('dialog', { name: 'Command palette' })
		.getByRole('option', { name: 'New recipe' })
		.click();
	await expect(page).toHaveURL('/recipes/new');
	await expect(page.getByRole('textbox', { name: 'Title', exact: true })).toHaveValue('');
});

for (const [name, link, kept] of [
	['keeps the link', 'https://blog.example/spaetzle', true],
	// A source over 500 characters would fail the save; the editor gets none.
	['drops a link too long to keep', `https://blog.example/${'x'.repeat(500)}`, false]
] as const) {
	test(`a page without a recipe takes the text instead and ${name}`, async ({ page }) => {
		// Only the link is refused; the text that follows reaches the real service.
		await page.route('**/api/v1/recipe-drafts', async (route) => {
			if (!route.request().postDataJSON().url) return route.fallback();
			await route.fulfill({
				status: 422,
				json: {
					title: 'Unprocessable Entity',
					status: 422,
					detail: 'import failed',
					errors: [{ location: 'body.url', message: 'no-recipe' }]
				}
			});
		});
		await openImport(page);
		await field(page).fill(link);
		await submit(page);

		await expect(dialog(page).getByRole('alert')).toContainText(
			'We found no recipe on blog.example.'
		);
		await expect(field(page)).toHaveValue('');
		await expect(dialog(page).getByText('blog.example', { exact: true })).toBeVisible();

		const title = `Spaetzle ${uniqueToken()}`;
		await field(page).fill(`${title}\nIngredients\n500 g flour\n5 eggs\nMethod\nBeat the dough.`);
		await submit(page);
		await expect(page).toHaveURL('/recipes/new');
		await expect(page.getByRole('textbox', { name: 'Title', exact: true })).toHaveValue(title);
		await expect(page.getByRole('textbox', { name: 'Link', exact: true })).toHaveValue(
			kept ? link : ''
		);
	});
}

test('the example shows in the empty field, and behind a toggle once text is in', async ({
	page
}) => {
	await openImport(page);
	await expect(field(page)).toHaveAttribute('placeholder', /Pancakes\nIngredients/);
	const toggle = dialog(page).getByRole('button', { name: 'What should the text look like?' });
	await expect(toggle).toHaveCount(0);

	await field(page).fill('Soup\nIngredients\n1 l water');
	await expect(toggle).toHaveAttribute('aria-expanded', 'false');
	await toggle.click();
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	await expect(dialog(page).locator('#import-example')).toContainText('200 g flour');

	// A link needs no example: toggle and example go.
	await field(page).fill('https://example.com/soup');
	await expect(toggle).toHaveCount(0);
	await expect(dialog(page).locator('#import-example')).toHaveCount(0);
	// Back to text, it starts closed.
	await field(page).fill('Soup\nIngredients\n1 l water');
	await expect(toggle).toHaveAttribute('aria-expanded', 'false');
});

// The member half - no zip row - is in transfer.test.ts, beside the page it guards.
test('an admin is offered the zip import in the dialog', async ({ page }) => {
	await openImport(page);
	await expect(dialog(page).getByRole('link', { name: 'Import a zip file' })).toHaveAttribute(
		'href',
		'/settings/transfer#import'
	);
});

test('the phone "+" offers writing or importing a recipe', async ({ page, isMobile }) => {
	test.skip(!isMobile, 'the "+" is the phone layout only');
	await page.getByRole('button', { name: 'New', exact: true }).click();
	const sheet = page.getByRole('dialog', { name: 'New recipe' });
	await expect(sheet.getByRole('button', { name: /^Write it yourself/ })).toBeVisible();
	await sheet.getByRole('button', { name: /^Import/ }).click();
	await expect(field(page)).toBeVisible();
});
