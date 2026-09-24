import { describe, expect, it } from 'vitest';
import { estimateStrength, strengthLabel, type Strength } from './password-strength';

describe('estimateStrength', () => {
	it('has nothing to say about an empty password', async () => {
		expect(await estimateStrength('')).toBeNull();
	});

	it('knows the common passwords', async () => {
		expect(await estimateStrength('password1')).toBe(0);
		expect(await estimateStrength('qwertzuiop')).toBeLessThanOrEqual(1);
	});

	it('rates a long random passphrase strong', async () => {
		expect(await estimateStrength('tangerine-orbit-velvet-harbor-91')).toBe(4);
	});

	it('marks a password down for containing the account name', async () => {
		const password = 'brunhilde1984';
		const without = await estimateStrength(password);
		const with_ = await estimateStrength(password, ['brunhilde']);
		expect(with_!).toBeLessThan(without!);
		expect(with_!).toBeLessThanOrEqual(1);
	});

	// The example docs/user/content/guide/settings.mdx gives.
	it('rates anna1990 weak for Anna', async () => {
		expect(await estimateStrength('anna1990', ['anna', 'Anna'])).toBeLessThanOrEqual(1);
	});

	it('matches each word of a display name on its own', async () => {
		const password = 'kowalczyk1984';
		const without = await estimateStrength(password);
		const with_ = await estimateStrength(password, ['Brunhilde Kowalczyk']);
		expect(with_!).toBeLessThan(without!);
		expect(with_!).toBeLessThanOrEqual(1);
	});

	it('marks a password down for naming the instance', async () => {
		expect(await estimateStrength('rezepte2024')).toBeLessThanOrEqual(1);
	});

	it('ignores blank user inputs', async () => {
		expect(await estimateStrength('tangerine-orbit-velvet-harbor-91', ['', '  '])).toBe(4);
	});
});

describe('strengthLabel', () => {
	it('names every score', () => {
		const labels = ([0, 1, 2, 3, 4] as Strength[]).map(strengthLabel);
		expect(new Set(labels).size).toBe(5);
		for (const label of labels) expect(label).not.toBe('');
	});
});
