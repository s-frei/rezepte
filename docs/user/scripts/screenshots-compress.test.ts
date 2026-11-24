import { expect, test } from 'bun:test';
import sharp from 'sharp';
import { isPalette, toPalette } from './screenshots-compress';

const truecolor = await sharp({ create: { width: 64, height: 64, channels: 3, background: '#d26b2c' } })
	.png()
	.toBuffer();

test('a render becomes a palette PNG, and the run after skips it', async () => {
	expect(isPalette(truecolor)).toBe(false);
	expect(isPalette(await toPalette(truecolor))).toBe(true);
});

test('the same render quantizes to the same bytes', async () => {
	expect((await toPalette(truecolor)).equals(await toPalette(truecolor))).toBe(true);
});
