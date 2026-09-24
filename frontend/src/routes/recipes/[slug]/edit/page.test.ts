import { isRedirect } from '@sveltejs/kit';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Recipe } from '$lib/api/recipes';
import { load } from './+page';

const loadRecipeBySlug = vi.fn();
vi.mock('$lib/recipe/load', () => ({
	loadRecipeBySlug: (slug: string) => loadRecipeBySlug(slug)
}));
vi.mock('$app/paths', () => ({
	resolve: (route: string, params: { slug: string }) => route.replace('[slug]', params.slug)
}));

type LoadEvent = Parameters<typeof load>[0];

async function run(slug: string) {
	return await load({ params: { slug } } as unknown as LoadEvent);
}

function recipe(canEdit: boolean): Pick<Recipe, 'slug' | 'canEdit'> {
	return { slug: 'zitronen-tarte', canEdit };
}

beforeEach(() => {
	loadRecipeBySlug.mockReset();
});

describe('edit route load', () => {
	it('sends someone who may not edit to the recipe', async () => {
		loadRecipeBySlug.mockResolvedValue(recipe(false));
		const error = await run('zitronen-tarte').then(
			() => null,
			(thrown: unknown) => thrown
		);
		expect(isRedirect(error)).toBe(true);
		expect(error).toMatchObject({ status: 307, location: '/recipes/zitronen-tarte' });
	});

	it('hands the recipe to the editor for someone who may edit', async () => {
		const loaded = recipe(true);
		loadRecipeBySlug.mockResolvedValue(loaded);
		await expect(run('zitronen-tarte')).resolves.toEqual({ recipe: loaded });
		expect(loadRecipeBySlug).toHaveBeenCalledWith('zitronen-tarte');
	});
});
