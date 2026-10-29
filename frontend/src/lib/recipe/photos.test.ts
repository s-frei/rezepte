import { beforeEach, describe, expect, it, vi } from 'vitest';
import { uploadCommentImage } from '#lib/api/comments.js';
import { stagePhotos, unstage, type PendingPhoto } from './photos';

vi.mock('#lib/api/comments.js', () => ({ uploadCommentImage: vi.fn() }));
const upload = vi.mocked(uploadCommentImage);

const staged = (id: string) => ({ id, width: 4, height: 3, inRecipe: false });
const pending = (key: string): PendingPhoto => ({
	key,
	file: new File(['x'], `${key}.png`, { type: 'image/png' }),
	preview: `blob:${key}`
});

describe('stagePhotos', () => {
	beforeEach(() => upload.mockReset());

	it('keeps what went up when one fails, and a retry uploads only the rest', async () => {
		const photos = [pending('a'), pending('b')];
		upload.mockResolvedValueOnce(staged('ia')).mockRejectedValueOnce(new Error('boom'));

		await expect(stagePhotos('r1', photos, () => {})).rejects.toThrow('boom');
		expect(photos[0].staged?.id).toBe('ia');
		expect(photos[1].failed).toBe(true);

		upload.mockResolvedValueOnce(staged('ib'));
		const progress: [number, number][] = [];
		const ids = await stagePhotos('r1', photos, (n, of) => progress.push([n, of]));

		expect(ids).toEqual(['ia', 'ib']);
		expect(upload).toHaveBeenCalledTimes(3);
		expect(upload).toHaveBeenLastCalledWith('r1', photos[1].file);
		expect(photos[1].failed).toBe(false);
		expect(progress).toEqual([[1, 1]]);
	});

	it('returns every id in order, already staged ones included, counting only uploads', async () => {
		const photos: PendingPhoto[] = [
			{ key: 'old', preview: '/thumb', staged: staged('old') },
			pending('a'),
			pending('b')
		];
		upload.mockResolvedValueOnce(staged('ia')).mockResolvedValueOnce(staged('ib'));
		const progress: [number, number][] = [];

		const ids = await stagePhotos('r1', photos, (n, of) => progress.push([n, of]));

		expect(ids).toEqual(['old', 'ia', 'ib']);
		expect(progress).toEqual([
			[1, 2],
			[2, 2]
		]);
	});

	it('after unstage a retry uploads every picked file again, not the existing photos', async () => {
		const photos: PendingPhoto[] = [
			{ key: 'old', preview: '/thumb', staged: staged('old') },
			{ ...pending('a'), staged: staged('gone') },
			{ ...pending('b'), failed: true }
		];
		unstage(photos);
		expect(photos.map((p) => [p.staged?.id, p.failed])).toEqual([
			['old', undefined],
			[undefined, false],
			[undefined, false]
		]);
		upload.mockResolvedValueOnce(staged('ia')).mockResolvedValueOnce(staged('ib'));
		expect(await stagePhotos('r1', photos, () => {})).toEqual(['old', 'ia', 'ib']);
	});
});
