import { describe, expect, test } from 'vitest';
import type { FormStep } from './form';
import {
	addPhoto,
	photoBlocker,
	photoIds,
	resolvePhotos,
	visiblePhotos,
	type PhotoTile
} from './step-photos';

const TEXT = 'Teig durch eine Spätzlepresse oder vom Brett ins kochende Salzwasser schaben.';

const step = (overrides: Partial<FormStep> = {}): FormStep => ({
	id: 's1',
	text: TEXT,
	references: [],
	times: [],
	photos: [],
	...overrides
});

const tile = (key: string): PhotoTile => ({ key, id: key, thumb: `${key}.t`, detail: `${key}.d` });

describe('photoBlocker', () => {
	test('refuses a phrase overlapping a stored time', () => {
		const s = step({
			text: 'Den Teig 10 Minuten ruhen lassen.',
			times: [{ phrase: '10 Minuten', seconds: 600 }]
		});
		expect(photoBlocker(s, 'Minuten ruhen')).toBe('time');
	});

	test('refuses a phrase overlapping another photo link', () => {
		const s = step({ photos: [{ phrase: 'kochende Salzwasser', imageKey: 'i1' }] });
		expect(photoBlocker(s, 'Salzwasser schaben')).toBe('photo');
	});

	test('lets a phrase be linked again, which changes its photo', () => {
		const s = step({ photos: [{ phrase: 'schaben', imageKey: 'i1' }] });
		expect(photoBlocker(s, 'schaben')).toBeNull();
	});

	test('allows a phrase containing a linked ingredient word', () => {
		const s = step({
			references: [
				{ word: 'Salzwasser', ingredientId: 'r1', groupName: null, ingredientName: 'Salz' }
			]
		});
		expect(photoBlocker(s, 'ins kochende Salzwasser schaben')).toBeNull();
	});
});

describe('addPhoto', () => {
	test('replaces the link on the same phrase', () => {
		const photos = addPhoto(
			[
				{ phrase: 'schaben', imageKey: 'i1' },
				{ phrase: 'Teig', imageKey: 'i2' }
			],
			{ phrase: 'schaben', imageKey: 'i3' },
			TEXT
		);
		expect(photos).toEqual([
			{ phrase: 'Teig', imageKey: 'i2' },
			{ phrase: 'schaben', imageKey: 'i3' }
		]);
	});

	test('keeps the links in the order their phrases stand in the text', () => {
		const photos = addPhoto(
			[{ phrase: 'schaben', imageKey: 'i1' }],
			{ phrase: 'Teig', imageKey: 'i2' },
			TEXT
		);
		expect(photos.map((photo) => photo.phrase)).toEqual(['Teig', 'schaben']);
	});
});

describe('photoIds', () => {
	const tiles = [
		{ ...tile('stored'), id: 'img1' },
		{ ...tile('uploading'), id: null }
	];

	test('an edited recipe sends only stored photos, never a key as an id', () => {
		expect(photoIds(tiles, false)).toEqual(new Map([['stored', 'img1']]));
	});

	test('a new recipe sends a queued photo by its key, for its save to resolve', () => {
		expect(photoIds(tiles, true)).toEqual(
			new Map([
				['stored', 'img1'],
				['uploading', 'uploading']
			])
		);
	});
});

describe('resolvePhotos', () => {
	const step = {
		text: 'Teig schaben.',
		photos: [
			{ phrase: 'Teig', imageId: 'k1' },
			{ phrase: 'schaben', imageId: 'k2' }
		]
	};

	test('swaps keys for ids and leaves out a link without one', () => {
		expect(resolvePhotos(step, new Map([['k2', 'img2']]))).toEqual({
			text: 'Teig schaben.',
			photos: [{ phrase: 'schaben', imageId: 'img2' }]
		});
	});

	test('leaves photos off a step with none left', () => {
		expect(resolvePhotos(step, new Map())).toEqual({ text: 'Teig schaben.' });
	});
});

describe('visiblePhotos', () => {
	test('hides a link whose photo is not among the tiles', () => {
		const photos = [
			{ phrase: 'schaben', imageKey: 'i1' },
			{ phrase: 'Teig', imageKey: 'gone' }
		];
		expect(visiblePhotos(photos, [tile('i1'), tile('i2')])).toEqual([photos[0]]);
	});
});
