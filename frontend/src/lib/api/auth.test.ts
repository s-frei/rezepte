import { describe, expect, it } from 'vitest';
import { safeNext } from './auth';

describe('safeNext', () => {
	it('defaults to the root', () => {
		expect(safeNext(null)).toBe('/');
		expect(safeNext('')).toBe('/');
	});
	it('keeps same-origin paths', () => {
		expect(safeNext('/recipes/pasta')).toBe('/recipes/pasta');
		expect(safeNext('/settings?tab=users')).toBe('/settings?tab=users');
	});
	it('rejects external and protocol-relative targets', () => {
		expect(safeNext('https://evil.example')).toBe('/');
		expect(safeNext('//evil.example')).toBe('/');
		expect(safeNext('/login')).toBe('/');
	});
	it('rejects host-embedding tricks the URL parser would otherwise fall for', () => {
		expect(safeNext('/\\evil.com')).toBe('/');
		expect(safeNext('/\t/evil.com')).toBe('/');
		expect(safeNext('/\n/evil.com')).toBe('/');
		expect(safeNext('//evil.com')).toBe('/');
		expect(safeNext('https://evil.example')).toBe('/');
		expect(safeNext('/login?x=1')).toBe('/');
	});
	it('keeps an unchanged same-origin path with query', () => {
		expect(safeNext('/recipes/pasta?tab=1')).toBe('/recipes/pasta?tab=1');
	});
});
