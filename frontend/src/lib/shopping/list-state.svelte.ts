import {
	getShoppingList,
	listEventsUrl,
	setItemsChecked,
	type ShoppingList
} from '#lib/api/shopping.js';
import { LiveState } from '#lib/live-state.svelte.js';
import type { Row } from './rows';

/** How long a row another device ticked stays in its place, ticked and lit, before it moves to Done. */
export const LINGER_MS = 1500;

/** A shopping list on a page: `LiveState`'s rules, plus ticking and the rows another device just ticked. */
export class ListState extends LiveState<ShoppingList> {
	/** Items another device just ticked (seen in a refresh): still shown in their place for a moment. */
	lingering = $state<string[]>([]);
	#lingerTimer: ReturnType<typeof setTimeout> | undefined;

	get list() {
		return this.value;
	}
	set list(next: ShoppingList) {
		this.value = next;
	}

	protected fetch() {
		return getShoppingList(this.list.id);
	}
	protected eventsUrl(id: string) {
		return listEventsUrl(id);
	}

	/**
	 * The list as the page shows it: an item in `lingering` still counts as
	 * open, so its row stays where it was; `rowsOf` marks the row, and
	 * `ShoppingRow` shows it ticked and lit.
	 */
	get shown(): ShoppingList {
		if (this.lingering.length === 0) return this.list;
		const ids = this.lingering;
		return {
			...this.list,
			items: this.list.items.map((i) =>
				ids.includes(i.id) ? { ...i, checked: false, lingering: true } : i
			)
		};
	}

	/** A refetched list; what it ticked since lingers in place for a moment. */
	protected take(answer: ShoppingList) {
		const open = this.list.items.filter((i) => !i.checked).map((i) => i.id);
		const ticked = answer.items.filter((i) => i.checked && open.includes(i.id)).map((i) => i.id);
		this.list = answer;
		if (ticked.length === 0) return;
		clearTimeout(this.#lingerTimer);
		this.lingering = [...this.lingering, ...ticked];
		this.#lingerTimer = setTimeout(() => (this.lingering = []), LINGER_MS);
	}

	#withChecked(ids: string[], checked: boolean): ShoppingList {
		return {
			...this.list,
			items: this.list.items.map((i) => (ids.includes(i.id) ? { ...i, checked } : i))
		};
	}

	/**
	 * Ticks or unticks a row's items: the circle flips at once, a failed request
	 * flips it back. A lingering row unticked stops lingering, or it would still show ticked.
	 */
	toggle(row: Pick<Row, 'checked' | 'itemIds'>) {
		const checked = !row.checked;
		this.lingering = this.lingering.filter((id) => !row.itemIds.includes(id));
		this.list = this.#withChecked(row.itemIds, checked);
		return this.apply(setItemsChecked(this.list.id, row.itemIds, checked), () =>
			this.#withChecked(row.itemIds, !checked)
		);
	}
}
