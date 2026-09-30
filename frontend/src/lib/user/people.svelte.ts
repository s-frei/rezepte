import { SvelteMap } from 'svelte/reactivity';
import { listAuthors } from '$lib/api/recipes';
import { listPeople, type UserRole } from '$lib/api/users';

/**
 * What a person card shows beyond the person a recipe already carries: the
 * role, from /people, and how many recipes they wrote, from /authors. Loaded
 * once, on the first card that opens, and shared by every card after it; a
 * reload of the page is what refreshes it, which is as fresh as a household
 * needs a recipe count to be.
 */
const state = $state({
	roles: new SvelteMap<string, UserRole>(),
	counts: new SvelteMap<string, number>(),
	loaded: false
});
let pending: Promise<void> | null = null;

export function personDetails() {
	return {
		get roles() {
			return state.roles;
		},
		get counts() {
			return state.counts;
		},
		get loaded() {
			return state.loaded;
		},
		load(): Promise<void> {
			pending ??= Promise.all([listPeople(), listAuthors()])
				.then(([people, authors]) => {
					state.roles = new SvelteMap(people.map((p) => [p.id, p.role]));
					state.counts = new SvelteMap(authors.map((a) => [a.username, a.count]));
					state.loaded = true;
				})
				.catch(() => {
					// A card without role and count is still a card; try again next time.
					pending = null;
				});
			return pending;
		}
	};
}
