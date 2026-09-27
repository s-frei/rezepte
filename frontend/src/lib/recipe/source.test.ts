import { describe, expect, it } from 'vitest';
import { sourceLabel } from './source';

describe('sourceLabel', () => {
	it('prefers the name', () => {
		expect(sourceLabel('Chefkoch', 'https://www.chefkoch.de/r/1')).toBe('Chefkoch');
	});

	it('trims the name', () => {
		expect(sourceLabel('  Tante Erika ', null)).toBe('Tante Erika');
	});

	it('falls back to the hostname without www', () => {
		expect(sourceLabel(null, 'https://www.chefkoch.de/r/1')).toBe('chefkoch.de');
		expect(sourceLabel('  ', 'https://localhost:8080/x')).toBe('localhost');
	});

	it('is null when there is nothing to credit', () => {
		expect(sourceLabel(null, null)).toBeNull();
		expect(sourceLabel('', 'not a url')).toBeNull();
	});
});
