import { expect, test, type Page } from '@playwright/test';
import { createRecipe, expectSpotlight, login, tinyPng, uniqueToken, uploadImage } from './helpers';

// A step's photo phrase opens its photo: a card on a desktop, a sheet on a
// phone (the mobile project), in the detail page and cook mode; the public
// link's photo is in public-shares.test.ts, beside the other tests that flip
// the instance-wide switch.

test.beforeEach(async ({ page }) => {
	await login(page);
});

const PHRASE = 'vom Brett ins kochende Salzwasser schaben';

async function recipeWithPhotos(
	page: Page,
	phrase = PHRASE,
	references: { word: string; groupName: null; ingredientName: string }[] = [],
	more = ''
) {
	const input = {
		title: `Foto ${uniqueToken()}`,
		description: '',
		servings: 4,
		prepMinutes: null,
		cookMinutes: null,
		sourceUrl: null,
		sourceName: null,
		tags: [],
		ingredientGroups: [
			{ name: null, ingredients: [{ quantity: 400, unit: 'g', name: 'Mehl', note: null }] }
		],
		steps: [
			{
				text: `Teig ${phrase}, dann 15 minutes ziehen lassen.${more}`,
				references,
				times: [{ phrase: '15 minutes', seconds: 900 }]
			},
			{ text: 'Servieren.', references: [], times: [] }
		],
		editPolicy: 'open' as const
	};
	const recipe = await createRecipe(page, input);
	const ids: string[] = [];
	for (let i = 0; i < 3; i++) {
		ids.push((await uploadImage(page, recipe.id, tinyPng([60 * i, 120, 200 - 40 * i]))).id);
	}
	const origin = new URL(page.url()).origin;
	const put = await page.request.put(`/api/v1/recipes/${recipe.id}`, {
		headers: { Origin: origin, 'Content-Type': 'application/json' },
		data: {
			...input,
			steps: [{ ...input.steps[0], photos: [{ phrase, imageId: ids[2] }] }, input.steps[1]]
		}
	});
	expect(put.ok(), `PUT recipe failed: ${put.status()} ${await put.text()}`).toBeTruthy();
	return { recipe, ids, origin };
}

const trigger = (page: Page) => page.locator('[data-step-photo]');

/**
 * The open sheet leaves the phrase in sight: its last line ends above the
 * sheet's resting top, its first line below the top of the step's scroll box
 * (cook mode) or of the window. Polled, as the page scrolls smoothly there.
 */
async function expectPhraseAboveSheet(page: Page) {
	await expect
		.poll(() =>
			page.evaluate(() => {
				const phrase = document.querySelector('[data-step-photo]')!;
				const rects = [...phrase.getClientRects()];
				const sheet = document.querySelector<HTMLElement>('[role="dialog"]')!;
				const top = window.innerHeight - sheet.offsetHeight;
				const box = phrase.closest('section[data-step]')?.getBoundingClientRect().top ?? 0;
				return rects[0].top >= box && rects.at(-1)!.bottom < top;
			})
		)
		.toBe(true);
}

test('the detail page opens the linked photo, large too', async ({ page, isMobile }) => {
	const { recipe, ids } = await recipeWithPhotos(page);
	await page.goto(`/recipes/${recipe.slug}`);

	await trigger(page).click();
	const preview = page.getByRole('dialog');
	await expect(preview.locator('img')).toHaveAttribute('src', new RegExp(ids[2]));
	await expect(preview.getByText('Photo 3 of 3')).toBeVisible();
	if (isMobile) {
		await expect(page.getByRole('dialog', { name: `Step 1 · ${PHRASE}` })).toBeVisible();
		await expectPhraseAboveSheet(page);
		await expectSpotlight(page.locator('li', { has: trigger(page) }));
	}

	await preview.getByRole('button', { name: 'View large' }).click();
	await expect(page.getByText('3 / 3')).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(page.getByText('3 / 3')).toHaveCount(0);
});

test('a press on the open phrase keeps its card, Escape returns focus to it', async ({
	page,
	isMobile
}) => {
	test.skip(isMobile, 'the card at the phrase is the desktop path');
	const { recipe } = await recipeWithPhotos(page);
	await page.goto(`/recipes/${recipe.slug}`);

	await trigger(page).click();
	await expect(page.getByRole('dialog')).toBeVisible();
	await trigger(page).click();
	await expect(page.getByRole('dialog')).toBeVisible();

	await page.keyboard.press('Escape');
	await expect(page.getByRole('dialog')).toHaveCount(0);
	await expect(trigger(page)).toBeFocused();

	await page.keyboard.press('Enter');
	await expect(page.getByRole('dialog')).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(trigger(page)).toBeFocused();

	// Space opens on keyup, like a native button: the keyup never reaches
	// "View large", which has focus by then.
	await page.keyboard.down(' ');
	await expect(page.getByRole('dialog')).toHaveCount(0);
	await page.keyboard.up(' ');
	await expect(page.getByRole('dialog')).toBeVisible();
	await expect(page.getByText('3 / 3')).toHaveCount(0);
});

test('the trigger is named with the quantities inside its phrase', async ({ page }) => {
	const { recipe } = await recipeWithPhotos(page, 'mit Mehl bestäuben', [
		{ word: 'Mehl', groupName: null, ingredientName: 'Mehl' }
	]);
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(trigger(page)).toHaveAccessibleName(/^Show photo: mit Mehl ?\(400 g\) bestäuben$/);
});

test('cook mode on a large screen shows the photo beside the step', async ({ page, isMobile }) => {
	test.skip(isMobile, 'a phone gets the sheet');
	await page.setViewportSize({ width: 1280, height: 800 });
	const { recipe, ids } = await recipeWithPhotos(page);
	await page.goto(`/recipes/${recipe.slug}/cook?step=1`);
	const panel = page.locator('[data-step-photo-panel]');
	const close = panel.getByRole('button', { name: 'Close photo' });

	await trigger(page).click();
	await expect(panel.locator('img')).toHaveAttribute('src', new RegExp(`${ids[2]}/detail`));
	await expect(panel.getByText('Photo 3 of 3')).toBeVisible();
	await expect(close).toBeFocused();
	// Beside the step, never over the footer or the ingredient bar.
	const box = (await panel.boundingBox())!;
	for (const covered of [
		page.getByRole('button', { name: 'Next', exact: true }),
		page.locator('button[aria-expanded="false"]', { hasText: 'Ingredients' })
	]) {
		const other = (await covered.boundingBox())!;
		expect(box.y + box.height <= other.y || other.y + other.height <= box.y).toBe(true);
	}

	await close.click();
	await expect(panel).toHaveCount(0);
	await expect(trigger(page)).toBeFocused();

	await trigger(page).click();
	await expect(panel).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(panel).toHaveCount(0);
	await expect(page).toHaveURL(/\/cook\?step=1/);

	await trigger(page).click();
	await expect(panel).toBeVisible();
	await trigger(page).click();
	await expect(panel).toHaveCount(0);

	await trigger(page).click();
	await panel.getByRole('button', { name: 'View large' }).click();
	await expect(page.getByText('3 / 3')).toBeVisible();
	await page.keyboard.press('Escape');

	// A window narrowed past the rule closes it rather than squeezing the text.
	await page.setViewportSize({ width: 900, height: 800 });
	await expect(panel).toHaveCount(0);
	await page.setViewportSize({ width: 1280, height: 800 });

	await trigger(page).click();
	await expect(panel).toBeVisible();
	await page.getByRole('button', { name: 'Next', exact: true }).click();
	await expect(page).toHaveURL(/\/cook\?step=2/);
	await expect(panel).toHaveCount(0);
});

test('cook mode falls back to the sheet where the photo does not fit beside', async ({
	page,
	isMobile
}) => {
	test.skip(isMobile, 'a phone always gets the sheet');
	await page.setViewportSize({ width: 900, height: 1200 });
	const { recipe } = await recipeWithPhotos(page);
	await page.goto(`/recipes/${recipe.slug}/cook?step=1`);
	await trigger(page).click();
	await expect(page.locator('[data-step-photo-panel]')).toHaveCount(0);
	await expect(page.getByRole('dialog').getByText(`Step 1 · ${PHRASE}`)).toBeVisible();
	await expectPhraseAboveSheet(page);
});

// Sentences that lengthen a step until it nearly fills cook mode's stage.
const more = (count: number) =>
	' Die Spätzle schwimmen nach kurzer Zeit oben; dann mit einer Schaumkelle herausheben, abtropfen lassen und in eine vorgewärmte Schüssel geben.'.repeat(
		count
	);

test('cook mode keeps the title of a step the photo sheet padded', async ({ page, isMobile }) => {
	test.skip(isMobile, 'sets its own viewport');
	await page.setViewportSize({ width: 900, height: 1200 });
	// Fits alone, but not above the sheet.
	const { recipe } = await recipeWithPhotos(page, PHRASE, [], more(4));
	await page.goto(`/recipes/${recipe.slug}/cook?step=1`);
	const title = page.getByRole('heading', { name: recipe.title });
	await expect(trigger(page)).toBeVisible();
	await expect(title).toBeVisible();
	await expect(page.locator('[data-whole-step]')).toHaveCount(0);

	await trigger(page).click();
	await expect(page.getByRole('dialog')).toBeVisible();
	// The sheet's padding makes the step scroll for now...
	await expect
		.poll(() =>
			page
				.locator('section[data-step]')
				.evaluate((node) => node.scrollHeight > node.clientHeight + 1)
		)
		.toBe(true);
	await page.keyboard.press('Escape');
	await expect(page.getByRole('dialog')).toHaveCount(0);
	// ...but the title steps back only for a step that is long on its own.
	await expect(title).toBeVisible();
	await expect(page.locator('[data-whole-step]')).toHaveCount(0);
});

test('the whole step closes the photo beside it, the title comes back after', async ({
	page,
	isMobile
}) => {
	test.skip(isMobile, 'a phone gets the sheet');
	await page.setViewportSize({ width: 1024, height: 800 });
	// Fits alone, but not in the narrower column beside the photo.
	const { recipe } = await recipeWithPhotos(page, PHRASE, [], more(2) + ' Sofort servieren.');
	await page.goto(`/recipes/${recipe.slug}/cook?step=1`);
	const panel = page.locator('[data-step-photo-panel]');
	const title = page.getByRole('heading', { name: recipe.title });
	await expect(title).toBeVisible();

	await trigger(page).click();
	await expect(panel).toBeVisible();
	// Beside the photo the text is narrow enough to scroll.
	await page.getByRole('button', { name: 'Whole step' }).click();
	await expect(panel).toHaveCount(0);
	await page.getByRole('button', { name: 'Close the whole step' }).click();
	await expect(title).toBeVisible();
});

test('cook mode opens it, and the time chip still starts its timer', async ({ page, isMobile }) => {
	const { recipe, ids } = await recipeWithPhotos(page);
	await page.goto(`/recipes/${recipe.slug}/cook?step=1`);

	await trigger(page).click();
	await expect(page.getByRole('dialog').locator('img')).toHaveAttribute('src', new RegExp(ids[2]));
	if (isMobile) await expectPhraseAboveSheet(page);
	await page.keyboard.press('Escape');
	await expect(page.getByRole('dialog')).toHaveCount(0);
	// Escape closed the preview only, not cook mode.
	await expect(page).toHaveURL(/\/cook\?step=1/);

	await page.getByRole('button', { name: /Start timer: 15 minutes/ }).click();
	await expect(page.locator('[data-timer-dock], [data-timer-indicator]').first()).toBeVisible();
});

test('a reordered photo keeps its link and shows its new position', async ({ page }) => {
	const { recipe, ids, origin } = await recipeWithPhotos(page);
	const order = await page.request.put(`/api/v1/recipes/${recipe.id}/images/order`, {
		headers: { Origin: origin, 'Content-Type': 'application/json' },
		data: { imageIds: [ids[2], ids[0], ids[1]] }
	});
	expect(order.ok()).toBeTruthy();
	await page.goto(`/recipes/${recipe.slug}`);

	await trigger(page).click();
	const preview = page.getByRole('dialog');
	await expect(preview.locator('img')).toHaveAttribute('src', new RegExp(ids[2]));
	await expect(preview.getByText('Photo 1 of 3')).toBeVisible();
});

test('a long phrase wraps inline at 320 px', async ({ page, isMobile }) => {
	test.skip(isMobile, 'sets its own viewport');
	const long =
		'Teigportionen vorsichtig vom Brett ins kochende Salzwasser schaben und ziehen lassen';
	const { recipe } = await recipeWithPhotos(page, long);
	await page.setViewportSize({ width: 320, height: 700 });
	await page.goto(`/recipes/${recipe.slug}`);
	await expect(trigger(page)).toBeVisible();
	expect(await trigger(page).evaluate((el) => el.getClientRects().length)).toBeGreaterThan(1);
	expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
		true
	);
});

test('a long first word hyphenates in cook mode at the largest size, 360 px', async ({
	page,
	isMobile
}) => {
	test.skip(isMobile, 'sets its own viewport');
	const { recipe } = await recipeWithPhotos(page, 'Spätzleteigportionen vom Brett schaben');
	await page.setViewportSize({ width: 360, height: 740 });
	await page.goto(`/recipes/${recipe.slug}/cook?step=1`);
	await page.getByRole('button', { name: 'Type size' }).click();
	await page.getByRole('radio', { name: 'Double Pica' }).click();
	await page.keyboard.press('Escape');
	await expect(trigger(page)).toBeVisible();
	const [scroll, client] = await page
		.locator('section[data-step]')
		.evaluate((node) => [node.scrollWidth, node.clientWidth]);
	expect(scroll).toBeLessThanOrEqual(client);
	const [right, column] = await trigger(page).evaluate((node) => [
		Math.max(...[...node.getClientRects()].map((rect) => rect.right)),
		node.closest('p')!.getBoundingClientRect().right
	]);
	expect(right).toBeLessThanOrEqual(column + 0.5);
});
