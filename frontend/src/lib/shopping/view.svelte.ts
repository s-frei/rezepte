import { load, save } from '#lib/storage.js';

/** How the list page groups its rows, remembered per device like the card size. */
export type ListView = 'department' | 'recipe';

const KEY = 'rezepte:list-view';

export const listView = $state({
	current: (load(KEY) === 'recipe' ? 'recipe' : 'department') as ListView
});

/** The reader's picks so far: a borrowed view gives way only to its own. */
let picks = 0;

export function setListView(view: ListView): void {
	picks++;
	listView.current = view;
	save(KEY, view);
}

/**
 * Shows `view` for one visit (a stamp's jump to a recipe's block), not
 * remembered; the returned function puts the reader's own view back, unless
 * they picked one meanwhile.
 */
export function borrowListView(view: ListView): () => void {
	const before = listView.current;
	const at = picks;
	listView.current = view;
	return () => {
		if (picks === at) listView.current = before;
	};
}
