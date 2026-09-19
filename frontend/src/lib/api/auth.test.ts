import { describe, expect, it } from 'vitest';
import { isAppPath, safeNext } from './auth';

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

describe('isAppPath', () => {
	it('is true for the SPA routes goto can reach', () => {
		expect(isAppPath('/')).toBe(true);
		expect(isAppPath('/recipes/pasta')).toBe(true);
		expect(isAppPath('/settings?tab=users')).toBe(true);
	});
	it('is false for the server routes only a full page load reaches', () => {
		// The docs page redirects here after login, and goto would render the
		// SPA's own 404 for it instead of loading the page from the server.
		expect(isAppPath('/api/v1/docs')).toBe(false);
		expect(isAppPath('/api/v1/openapi.json')).toBe(false);
	});
	it('is not fooled by a recipe whose slug merely starts with api', () => {
		expect(isAppPath('/recipes/apis-and-sauces')).toBe(true);
		expect(isAppPath('/apifoo')).toBe(true);
	});
});
