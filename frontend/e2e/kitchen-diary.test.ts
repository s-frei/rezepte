import { expect, test, type Page, type TestInfo } from '@playwright/test';
import { createRecipe, createUser, loadFixture, login, signOut, uniqueToken } from './helpers';

/**
 * Signs whoever is in out and `username` in, from the overview: the recipe
 * page hides the phone's bottom nav, which holds the sign-out.
 */
async function switchTo(page: Page, testInfo: TestInfo, username: string) {
	await page.goto('/');
	await signOut(page, testInfo);
	await login(page, username);
	await expect(page).toHaveURL('/');
}

const diary = (page: Page) => page.getByRole('tabpanel', { name: /^Comments/ });
const diaryTab = (page: Page) => page.getByRole('tab', { name: /^Comments/ });
const dot = (page: Page) => page.getByRole('button', { name: /new comments/ });

/** Opens the recipe page, then its diary tab, and waits until that marked the diary seen. */
async function openRecipe(page: Page, slug: string) {
	await page.goto(`/recipes/${slug}`);
	const seen = page.waitForResponse(
		(r) => r.url().endsWith('/comments/seen') && r.request().method() === 'PUT'
	);
	await diaryTab(page).click();
	expect((await seen).status()).toBe(204);
}

test('members talk in the comments and the author sees the dot', async ({ page }, testInfo) => {
	const token = uniqueToken();
	const mara = `mara${token}`.slice(0, 20);
	await login(page);
	await expect(page).toHaveURL('/');
	await createUser(page, { username: mara, role: 'user' });
	const recipe = await createRecipe(page, { ...loadFixture(1), title: `Diary ${token}` });

	// A member writes on somebody else's recipe, then edits it in place.
	// An empty diary has nothing to mark seen, opened or not.
	await switchTo(page, testInfo, mara);
	let seenCalls = 0;
	const count = (r: { url(): string; method(): string }) => {
		if (r.url().endsWith('/comments/seen') && r.method() === 'PUT') seenCalls++;
	};
	page.on('request', count);
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	expect(seenCalls).toBe(0);
	await diaryTab(page).click();
	await expect(diary(page).getByText('No comments yet')).toBeVisible();
	expect(seenCalls).toBe(0);
	page.off('request', count);
	await expect(diaryTab(page)).toHaveAccessibleName(/No comments yet/);
	// With a keyboard Shift+Enter breaks the line and Enter writes the entry;
	// on a touch screen Enter breaks the line and the send button writes.
	const touch = testInfo.project.name === 'mobile';
	const composer = diary(page).getByPlaceholder('What did you notice?');
	await composer.fill('Capers only');
	// An IME's Enter only confirms its composition: nothing is written (a
	// second entry would break the single-entry checks below).
	await composer.dispatchEvent('keydown', { key: 'Enter', isComposing: true });
	await expect(composer).toHaveValue('Capers only');
	await composer.press(touch ? 'Enter' : 'Shift+Enter');
	await composer.pressSequentially('at the very end.');
	await expect(composer).toHaveValue('Capers only\nat the very end.');
	if (touch) {
		await diary(page).getByRole('button', { name: 'Send' }).click();
	} else {
		await composer.press('Enter');
	}
	const entry = diary(page).getByRole('listitem');
	await expect(entry).toHaveText(/Capers only\s+at the very end\./);
	await expect(diaryTab(page)).toHaveAccessibleName(/1 comment/);
	await expect(composer).toHaveValue('');
	// Ready for the next one, keyboard still up on a phone.
	await expect(composer).toBeFocused();
	await expect(diary(page).getByText('Comment sent')).toBeAttached();

	await entry.getByRole('button', { name: 'Comment options' }).click();
	await page.getByRole('menuitem', { name: 'Edit' }).click();
	const field = entry.getByRole('textbox');
	await expect(field).toBeFocused();
	await expect(field).toHaveValue('Capers only\nat the very end.');
	await field.fill('Capers only at the very end, or they go mushy.');
	if (touch) {
		await field.press('Enter');
		await expect(field).toHaveValue('Capers only at the very end, or they go mushy.\n');
		await field.press('Backspace');
		await entry.getByRole('button', { name: 'Save' }).click();
	} else {
		await field.press('Enter');
	}
	await expect(entry).toHaveText(/or they go mushy\./);
	await expect(entry.getByText('edited')).toBeVisible();

	// The recipe's author sees the dot on the card until opening the diary;
	// the tab carries it too, and drops it once opened - once a visit.
	await switchTo(page, testInfo, 'admin');
	await page.goto(`/?q=${token}`);
	await expect(dot(page)).toBeVisible();
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(diaryTab(page)).toHaveAccessibleName(/1 comment\s*New/);
	await page.goto(`/?q=${token}`);
	await expect(dot(page)).toBeVisible();
	await openRecipe(page, recipe.slug);
	await expect(entry.getByText('New', { exact: true })).toBeAttached();
	await expect(diaryTab(page)).not.toHaveAccessibleName(/New/);
	page.on('request', count);
	await page.getByRole('tab', { name: 'Method' }).click();
	await diaryTab(page).click();
	await expect(diary(page)).toBeVisible();
	expect(seenCalls).toBe(0);
	page.off('request', count);
	await page.goto(`/?q=${token}`);
	await expect(page.getByRole('heading', { level: 3, name: `Diary ${token}` })).toBeVisible();
	await expect(dot(page)).toHaveCount(0);

	// The author answers with Ctrl/Cmd+Enter; an own entry is never new.
	await openRecipe(page, recipe.slug);
	await expect(entry.getByText('New', { exact: true })).toHaveCount(0);
	await composer.fill('Noted, thanks.');
	await composer.press('ControlOrMeta+Enter');
	await expect(diaryTab(page)).toHaveAccessibleName(/2 comments/);

	// An admin deletes somebody else's entry, after confirming.
	await diary(page)
		.getByRole('listitem')
		.filter({ hasText: 'mushy' })
		.getByRole('button', { name: 'Comment options' })
		.click();
	await expect(page.getByRole('menuitem', { name: 'Edit' })).toHaveCount(0);
	await page.getByRole('menuitem', { name: 'Delete' }).click();
	await page.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
	await expect(diary(page).getByText('mushy')).toHaveCount(0);
	await expect(diaryTab(page)).toHaveAccessibleName(/1 comment/);
	// The removed entry's menu held the focus; the composer takes it.
	await expect(composer).toBeFocused();

	// Her only entry is gone, and with it her part in the conversation: the
	// author's answer brings her no dot.
	await switchTo(page, testInfo, mara);
	await page.goto(`/?q=${token}`);
	await expect(page.getByRole('heading', { level: 3, name: `Diary ${token}` })).toBeVisible();
	await expect(dot(page)).toHaveCount(0);
});

test('a tablet sets a long ingredient list in two columns, a short one in one', async ({
	page
}) => {
	const token = uniqueToken();
	await page.setViewportSize({ width: 720, height: 1000 });
	await login(page);
	await expect(page).toHaveURL('/');
	const fixture = loadFixture(1);
	const long = await createRecipe(page, { ...fixture, title: `Columns ${token}` });
	const short = await createRecipe(page, {
		...fixture,
		title: `Column ${token}`,
		ingredientGroups: [
			{ name: null, ingredients: fixture.ingredientGroups[0].ingredients.slice(0, 3) }
		],
		// The steps name ingredients the short list no longer has.
		steps: fixture.steps.map((step) => ({ ...step, references: [] }))
	});
	const lefts = async () =>
		new Set(
			await page
				.getByRole('checkbox')
				.evaluateAll((boxes) => boxes.map((b) => Math.round(b.getBoundingClientRect().left)))
		).size;

	await page.goto(`/recipes/${long.slug}`);
	await expect(page.getByRole('checkbox').first()).toBeVisible();
	expect(await lefts()).toBe(2);
	// Checking off works in the right column too.
	const last = page.getByRole('checkbox').last();
	await last.click();
	await expect(last).toBeChecked();

	await page.goto(`/recipes/${short.slug}`);
	await expect(page.getByRole('checkbox').first()).toBeVisible();
	expect(await lefts()).toBe(1);
});
