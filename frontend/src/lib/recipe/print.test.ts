import { describe, expect, it, vi } from 'vitest';
import type { Recipe } from '../api/recipes';
import { printRoute, recipeRequest } from './print';

const ua = {
	chrome:
		'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36',
	edge: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36 Edg/142.0.0.0',
	firefox: 'Mozilla/5.0 (X11; Linux x86_64; rv:145.0) Gecko/20100101 Firefox/145.0',
	safari:
		'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.1 Safari/605.1.15',
	iphone:
		'Mozilla/5.0 (iPhone; CPU iPhone OS 26_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.1 Mobile/15E148 Safari/604.1',
	iosChrome:
		'Mozilla/5.0 (iPhone; CPU iPhone OS 26_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/142.0.0.0 Mobile/15E148 Safari/604.1',
	android:
		'Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Mobile Safari/537.36',
	androidFirefox: 'Mozilla/5.0 (Android 15; Mobile; rv:145.0) Gecko/145.0 Firefox/145.0'
};

describe('printRoute', () => {
	const route = (userAgent: string, pdfViewerEnabled = true) =>
		printRoute({ userAgent, pdfViewerEnabled });

	it('prints in a frame in desktop Chromium', () => {
		expect(route(ua.chrome)).toBe('frame');
		expect(route(ua.edge)).toBe('frame');
	});

	it('opens a tab in Firefox, Safari and on phones with a PDF viewer', () => {
		expect(route(ua.firefox)).toBe('tab');
		expect(route(ua.safari)).toBe('tab');
		expect(route(ua.iphone)).toBe('tab');
		expect(route(ua.iosChrome)).toBe('tab');
		expect(route(ua.androidFirefox)).toBe('tab');
	});

	it('downloads without a PDF viewer', () => {
		expect(route(ua.android, false)).toBe('download');
		expect(route(ua.chrome, false)).toBe('download');
	});
});

describe('recipeRequest', () => {
	it('asks for the given options at the servings', () => {
		const recipe = {
			id: 'r1',
			title: 'Soup',
			servings: 4,
			prepMinutes: null,
			cookMinutes: null,
			sourceUrl: null,
			sourceName: null,
			tags: [],
			ingredientGroups: [],
			steps: []
		} as unknown as Recipe;
		vi.stubGlobal('location', { origin: 'https://rezepte.example' });
		const options = { format: 'a5', photo: false, footer: true } as const;
		const body = recipeRequest(recipe, 2, options);
		expect(body.options).toEqual({
			bundle: 'single',
			paper: 'a5',
			photo: false,
			footer: true
		});
		expect(body.recipes[0].servings.value).toBe(2);
		expect(body.origin).toBe('https://rezepte.example');
		vi.unstubAllGlobals();
	});
});
