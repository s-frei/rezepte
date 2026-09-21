import { readFileSync } from 'node:fs';
import { deflateSync } from 'node:zlib';
import { expect, type Page, type TestInfo } from '@playwright/test';
// Type-only, so the relative hop into `src` is erased at runtime and the e2e
// suite still speaks the same contract as the app.
import type { Image, Recipe, RecipeInput } from '../src/lib/api/recipes';
import type { UserAccount } from '../src/lib/api/users';

/**
 * The password of every user this suite bootstraps: the username with `1234`
 * appended, the repository's rule for development credentials (see
 * docs/memory/content/conventions/dev-credentials.mdx). Derived rather than
 * written out, so no test has to invent one and none has to be looked up.
 */
export function devPassword(username: string): string {
	return `${username}1234`;
}

/** The replacement password where a test rotates one. Same rule, `5678`. */
export function devPasswordNext(username: string): string {
	return `${username}5678`;
}

/** Logs in as the instance owner `admin`, or as any bootstrapped user. */
export async function login(page: Page, username = 'admin', password = devPassword(username)) {
	await page.goto('/login');
	await page.getByLabel('Benutzername').fill(username);
	await page.getByLabel('Passwort').fill(password);
	await page.getByRole('button', { name: 'Anmelden' }).click();
}

/**
 * Signs out through the menu that holds "Einstellungen" and "Abmelden". The
 * two viewports build it differently - a Bits UI dropdown behind the avatar on
 * desktop, whose entries are `menuitem`s, and the "Mehr" sheet with real
 * buttons on phones, since the top bar is `md:` only - so the walk depends on
 * the project the test runs in.
 */
export async function signOut(page: Page, testInfo: TestInfo): Promise<void> {
	const mobile = testInfo.project.name.startsWith('mobile');
	// exact: true on the phone bottom-nav button - without it, "Mehr" also
	// matches the overview's "Mehr laden" button once enough recipes have
	// piled up in the shared database for the grid to paginate, which turns
	// this into a strict-mode violation (two matching buttons at once).
	await page.getByRole('button', { name: mobile ? 'Mehr' : 'Benutzermenü', exact: mobile }).click();
	await page.getByRole(mobile ? 'button' : 'menuitem', { name: 'Abmelden' }).click();
}

/**
 * A token unique to one test, used to keep recipes apart: the whole suite
 * runs against a single binary with a single database, and the desktop and
 * mobile projects run at the same time, so every test titles and tags its
 * own recipes with one of these instead of assuming it is alone.
 *
 * Starts with `e2e` so it never collides with the nonsense terms tests search
 * for to reach the no-results state.
 */
export function uniqueToken(): string {
	return `e2e${Math.random().toString(36).slice(2, 8)}${Date.now().toString(36).slice(-4)}`;
}

let fixtures: RecipeInput[] | null = null;

/**
 * Returns a copy of one of the German recipes the service tests are seeded
 * with (`service/internal/recipe/testdata/recipes.de.json`), so the e2e suite
 * exercises the same realistic data - umlauts, ingredient groups and all. The
 * suite's selectors are German, so this deliberately reads the German fixture
 * regardless of which locale a demo instance would seed.
 */
export function loadFixture(index: number): RecipeInput {
	if (fixtures === null) {
		const file = new URL('../../service/internal/recipe/testdata/recipes.de.json', import.meta.url);
		fixtures = JSON.parse(readFileSync(file, 'utf8')) as RecipeInput[];
	}
	const fixture = fixtures[index];
	if (!fixture) {
		throw new Error(`no recipe fixture at index ${index}`);
	}
	return structuredClone(fixture);
}

/**
 * Creates a recipe straight through the API, reusing the session cookie the
 * page logged in with (`page.request` shares the browser context's cookie
 * jar). Sets `Origin` the way the browser would, since the server rejects
 * mutating API calls whose origin doesn't match its host.
 *
 * Use it to arrange state for a test; the editor flow itself is covered
 * through the UI.
 */
export async function createRecipe(page: Page, input: RecipeInput): Promise<Recipe> {
	const origin = new URL(page.url()).origin;
	const response = await page.request.post('/api/v1/recipes', {
		headers: { Origin: origin, 'Content-Type': 'application/json' },
		data: input
	});
	expect(
		response.ok(),
		`POST /api/v1/recipes failed: ${response.status()} ${await response.text()}`
	).toBeTruthy();
	return (await response.json()) as Recipe;
}

/** Opens the recipe editor through whichever "new recipe" entry point the viewport shows. */
export async function openNewRecipe(page: Page) {
	const topBar = page.getByRole('link', { name: 'Neues Rezept' });
	const bottomNav = page.getByRole('link', { name: 'Neu', exact: true });
	await topBar.or(bottomNav).first().click();
	await expect(page).toHaveURL(/\/recipes\/new$/);
}

/** Types into the overview's search field and waits out its 250ms debounce. */
export async function search(page: Page, term: string) {
	const field = page.getByRole('textbox', { name: 'Rezepte durchsuchen' });
	await field.fill(term);
	await expect(field).toHaveValue(term);
}

/** Opens the detail page's "..." menu - the only entry point both viewports share. */
export async function openRecipeMenu(page: Page) {
	await page.getByRole('button', { name: 'Weitere Aktionen' }).click();
	await expect(page.getByRole('menu')).toBeVisible();
}

/**
 * A valid 64×64 RGB PNG built in-process (signature, IHDR, one deflated
 * IDAT, IEND), so the suite needs no binary fixtures. `tint` picks the
 * colour so two uploads are distinguishable in a screenshot.
 *
 * 64 px is deliberate: the service rejects anything smaller than 64 px on
 * either side (`image.minSide`), so this is the cheapest picture it accepts.
 */
export function tinyPng(tint: [number, number, number] = [200, 120, 40]): Buffer {
	const size = 64;
	const raw = Buffer.alloc(size * (1 + size * 3));
	for (let y = 0; y < size; y++) {
		const row = y * (1 + size * 3);
		raw[row] = 0; // filter: none
		for (let x = 0; x < size; x++) {
			raw.set(tint, row + 1 + x * 3);
		}
	}
	const chunk = (type: string, data: Buffer) => {
		const typeAndData = Buffer.concat([Buffer.from(type, 'ascii'), data]);
		const out = Buffer.alloc(8 + data.length + 4);
		out.writeUInt32BE(data.length, 0);
		typeAndData.copy(out, 4);
		out.writeUInt32BE(crc32(typeAndData), 8 + data.length);
		return out;
	};
	const ihdr = Buffer.alloc(13);
	ihdr.writeUInt32BE(size, 0);
	ihdr.writeUInt32BE(size, 4);
	ihdr.set([8, 2, 0, 0, 0], 8); // depth 8, RGB, deflate, filter 0, no interlace
	return Buffer.concat([
		Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
		chunk('IHDR', ihdr),
		chunk('IDAT', deflateSync(raw)),
		chunk('IEND', Buffer.alloc(0))
	]);
}

// CRC-32 (IEEE) as PNG requires; small enough to inline rather than pull a
// dependency or lean on node:zlib's crc32, which older Node versions lack.
const CRC_TABLE = new Uint32Array(256).map((_, n) => {
	let c = n;
	for (let k = 0; k < 8; k++) {
		c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
	}
	return c >>> 0;
});

function crc32(data: Buffer): number {
	let crc = 0xffffffff;
	for (const byte of data) {
		crc = CRC_TABLE[(crc ^ byte) & 0xff] ^ (crc >>> 8);
	}
	return (crc ^ 0xffffffff) >>> 0;
}

/** Makes one of a recipe's images its cover, through the API with the page's session. */
export async function setCover(page: Page, recipeId: string, imageId: string): Promise<void> {
	const origin = new URL(page.url()).origin;
	const response = await page.request.put(`/api/v1/recipes/${recipeId}/cover`, {
		headers: { Origin: origin, 'Content-Type': 'application/json' },
		data: { imageId }
	});
	expect(
		response.ok(),
		`PUT /api/v1/recipes/${recipeId}/cover failed: ${response.status()} ${await response.text()}`
	).toBeTruthy();
}

/** Uploads a PNG through the API with the page's session (multipart, part "file"). */
export async function uploadImage(page: Page, recipeId: string, png: Buffer): Promise<Image> {
	const origin = new URL(page.url()).origin;
	const response = await page.request.post(`/api/v1/recipes/${recipeId}/images`, {
		headers: { Origin: origin },
		multipart: { file: { name: 'photo.png', mimeType: 'image/png', buffer: png } }
	});
	expect(
		response.ok(),
		`POST /api/v1/recipes/${recipeId}/images failed: ${response.status()} ${await response.text()}`
	).toBeTruthy();
	return (await response.json()) as Image;
}

/** Creates a user through the API as whoever `page` is logged in as (an admin). */
export async function createUser(
	page: Page,
	input: { username: string; password?: string; role: 'admin' | 'user' }
): Promise<UserAccount> {
	const origin = new URL(page.url()).origin;
	const response = await page.request.post('/api/v1/users', {
		headers: { Origin: origin, 'Content-Type': 'application/json' },
		data: { ...input, password: input.password ?? devPassword(input.username) }
	});
	expect(
		response.ok(),
		`POST /api/v1/users failed: ${response.status()} ${await response.text()}`
	).toBeTruthy();
	return (await response.json()) as UserAccount;
}
