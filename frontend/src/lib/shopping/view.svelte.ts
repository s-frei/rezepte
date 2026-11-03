import { load, save } from '#lib/storage.js';

/** How the list page groups its rows, remembered per device like the card size. */
export type ListView = 'department' | 'recipe';

const KEY = 'rezepte:list-view';

export const listView = $state({
	current: (load(KEY) === 'recipe' ? 'recipe' : 'department') as ListView
});

export function setListView(view: ListView): void {
	listView.current = view;
	save(KEY, view);
}
