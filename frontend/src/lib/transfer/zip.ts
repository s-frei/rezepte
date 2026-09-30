// Reads and repacks the zip format of service/internal/transfer in the
// browser, so the import list needs no upload and only the chosen recipes
// are sent.
import { strFromU8, unzipSync, zipSync, type Zippable } from 'fflate';

export const FORMAT = 'rezepte.recipe';
export const VERSION = 1;

export type ImportRow = {
	folder: string;
	title: string;
	tags: string[];
	imageCount: number;
	cover: Uint8Array | null;
	error: 'unknown-format' | 'unsupported-version' | 'newer-version' | 'unreadable' | null;
};

const RECIPE_JSON = /^([^/]+)\/recipe\.json$/;

/**
 * Lists the recipes of an export. Only each folder's recipe.json and its
 * cover photo are inflated; the other photos stay unread until import.
 */
export function readExport(bytes: Uint8Array): ImportRow[] {
	// macOS Finder adds __MACOSX/ when it zips a folder.
	const jsons = unzipSync(bytes, {
		filter: (f) => !f.name.startsWith('__MACOSX/') && RECIPE_JSON.test(f.name)
	});
	const rows: ImportRow[] = [];
	const coverOf: Record<string, string> = {};
	for (const [name, json] of Object.entries(jsons)) {
		const folder = RECIPE_JSON.exec(name)![1];
		const { row, cover } = toRow(folder, json);
		rows.push(row);
		if (cover) coverOf[folder] = `${folder}/${cover}`;
	}
	const wanted = new Set(Object.values(coverOf));
	if (wanted.size === 0) return rows;
	const covers = unzipSync(bytes, { filter: (f) => wanted.has(f.name) });
	for (const row of rows) row.cover = covers[coverOf[row.folder]] ?? null;
	return rows;
}

function toRow(folder: string, json: Uint8Array): { row: ImportRow; cover: string | null } {
	const row: ImportRow = {
		folder,
		title: folder,
		tags: [],
		imageCount: 0,
		cover: null,
		error: null
	};
	let data: Record<string, unknown>;
	try {
		data = JSON.parse(strFromU8(json));
	} catch {
		return { row: { ...row, error: 'unreadable' }, cover: null };
	}
	// Valid JSON is not yet an object: null, 3 or [] would throw below.
	if (typeof data !== 'object' || data === null || Array.isArray(data))
		return { row: { ...row, error: 'unreadable' }, cover: null };
	if (typeof data.title === 'string') row.title = data.title;
	if (Array.isArray(data.tags)) row.tags = data.tags.filter((t) => typeof t === 'string');
	if (Array.isArray(data.images)) row.imageCount = data.images.length;
	const cover = typeof data.cover === 'string' ? data.cover : null;
	if (data.format !== FORMAT) row.error = 'unknown-format';
	else if (typeof data.version !== 'number' || data.version < 1) row.error = 'unsupported-version';
	else if (data.version > VERSION) row.error = 'newer-version';
	return { row, cover };
}

/**
 * A zip holding one folder of the export, stored rather than deflated: the
 * photos are JPEGs already. Only that folder is inflated.
 */
export function packFolder(bytes: Uint8Array, folder: string): Uint8Array {
	const files = unzipSync(bytes, { filter: (f) => f.name.startsWith(`${folder}/`) });
	const pick: Zippable = {};
	for (const [name, data] of Object.entries(files)) pick[name] = [data, { level: 0 }];
	return zipSync(pick);
}

/** What "already here" compares: the title, trimmed, case-insensitive. */
export function normalizeTitle(title: string): string {
	return title.trim().toLocaleLowerCase();
}
