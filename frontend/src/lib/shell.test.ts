import { describe, expect, it } from 'vitest';
import { NAV_PLACES, navPlace } from './shell.svelte';

/** Every page route's id, the way SvelteKit names it in `page.route.id`. */
const routeIds = Object.keys(import.meta.glob('/src/routes/**/+page.svelte')).map(
	(path) => path.replace('/src/routes', '').replace('/+page.svelte', '') || '/'
);

describe('navPlace', () => {
	it('marks the overview as Recipes', () => {
		expect(navPlace('/', false)).toBe('recipes');
	});

	it('marks every settings page as You', () => {
		expect(navPlace('/settings', false)).toBe('you');
		expect(navPlace('/settings/mail', false)).toBe('you');
	});

	it('shows no bar on a page with its own action bar', () => {
		expect(navPlace('/recipes/[slug]/edit', false)).toBeNull();
	});

	it('shows no bar on a path no route matches', () => {
		expect(navPlace(null, false)).toBeNull();
	});

	it('shows no bar while the error page is up', () => {
		expect(navPlace('/', true)).toBeNull();
	});

	// A new route has to decide whether it carries the bar; left out, it
	// would fall back to no bar without anyone having chosen that.
	it('every page route is in the nav table', () => {
		expect(routeIds.length).toBeGreaterThan(10);
		for (const id of routeIds) expect(Object.hasOwn(NAV_PLACES, id), id).toBe(true);
	});
});
