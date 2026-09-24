import { expect, test } from 'bun:test';
import sharp from 'sharp';
import { BORDER, rounded } from './readme-images';

async function pixel(png: Buffer, x: number, y: number): Promise<number[]> {
	const { data, info } = await sharp(png).ensureAlpha().raw().toBuffer({ resolveWithObject: true });
	const i = (y * info.width + x) * 4;
	return [...data.subarray(i, i + 4)];
}

const source = await sharp({
	create: { width: 400, height: 300, channels: 4, background: { r: 10, g: 20, b: 30, alpha: 1 } }
})
	.png()
	.toBuffer();

test('rounds the corners to transparent and keeps the size', async () => {
	const out = await rounded(source, 'light');
	const meta = await sharp(out).metadata();
	expect([meta.width, meta.height]).toEqual([400, 300]);
	expect((await pixel(out, 0, 0))[3]).toBe(0);
	expect((await pixel(out, 399, 299))[3]).toBe(0);
});

test('keeps the picture inside and draws the border in the theme color', async () => {
	const light = await rounded(source, 'light');
	expect(await pixel(light, 200, 150)).toEqual([10, 20, 30, 255]);
	const hex = (p: number[]) => `#${p.slice(0, 3).map((c) => c.toString(16).padStart(2, '0')).join('')}`;
	expect(hex(await pixel(light, 200, 0))).toBe(BORDER.light);
	expect(hex(await pixel(await rounded(source, 'dark'), 200, 0))).toBe(BORDER.dark);
});

const readme = await Bun.file(new URL('../../../README.md', import.meta.url)).text();

test('the README shows the rounded overview in both schemes', async () => {
	for (const scheme of ['light', 'dark']) {
		const name = `assets/readme/overview-desktop-${scheme}.png`;
		expect(readme).toContain(name);
		const committed = await sharp(await Bun.file(new URL(`../../../${name}`, import.meta.url)).arrayBuffer()).metadata();
		expect(committed.hasAlpha).toBe(true);
		expect([committed.width, committed.height]).toEqual([1440, 900]);
	}
});
