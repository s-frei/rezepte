import { describe, expect, it } from 'vitest';
import { avatarUrl, cropQuery } from './avatar';

describe('avatarUrl', () => {
	it('builds the file route from the ids', () => {
		expect(avatarUrl({ id: 'u1', avatarId: 'a1' })).toBe('/avatars/u1/a1.jpg');
	});
	it('is null without a picture', () => {
		expect(avatarUrl({ id: 'u1', avatarId: null })).toBeNull();
		expect(avatarUrl({ id: 'u1' })).toBeNull();
	});
});

describe('cropQuery', () => {
	it('writes x,y,size with four decimals', () => {
		expect(cropQuery({ x: 0.1, y: 1 / 3, size: 0.5 })).toBe('?crop=0.1000,0.3333,0.5000');
	});
});
