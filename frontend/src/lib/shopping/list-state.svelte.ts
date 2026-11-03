import { getShoppingList, type ShoppingList } from '#lib/api/shopping.js';

/**
 * A shopping list on a page, and the rules for changing it.
 *
 * Changes race: two quick taps send two requests, and their answers may
 * arrive in either order. Every change takes the next revision and only the
 * newest one's answer replaces the list; an older answer is dropped. Answers
 * for another list are dropped too - the page component stays mounted when
 * one list links to the next, so the revision moves on with each load as well.
 */
export class ListState {
	list = $state<ShoppingList>() as ShoppingList;
	#rev = 0;

	/** `onfail` is the page's own error handling (toast, redirect when the list is gone). */
	constructor(
		initial: ShoppingList,
		public onfail: (error: unknown) => void,
		private fetchList: (id: string) => Promise<ShoppingList> = getShoppingList
	) {
		this.list = initial;
	}

	/** A new page load: its list replaces the current one and outranks answers in flight. */
	load(next: ShoppingList) {
		this.#rev++;
		this.list = next;
	}

	/** Starts a change of the caller's own (an optimistic step); its answer outranks older ones. */
	begin() {
		return ++this.#rev;
	}

	isLatest(mine: number) {
		return mine === this.#rev;
	}

	#current(answer: ShoppingList, mine: number) {
		return mine === this.#rev && answer.id === this.list.id;
	}

	/** Reloads the list unless a newer change has started meanwhile (its answer will do). */
	async refresh() {
		const mine = this.#rev;
		try {
			const answer = await this.fetchList(this.list.id);
			if (this.#current(answer, mine)) this.list = answer;
		} catch (error) {
			this.onfail(error);
		}
	}

	/**
	 * Takes the server's answer as the new list when it is still the newest;
	 * false (and `onfail`) when the request failed. A failed change undoes its
	 * own optimistic step (`revert`) only while nothing newer was sent;
	 * otherwise the list is reloaded, since undoing locally would also undo
	 * the newer change.
	 */
	async apply(request: Promise<ShoppingList>, revert?: () => ShoppingList): Promise<boolean> {
		const mine = this.begin();
		const id = this.list.id;
		try {
			const answer = await request;
			if (this.#current(answer, mine)) this.list = answer;
			return true;
		} catch (error) {
			this.onfail(error);
			if (id !== this.list.id) return false;
			if (!this.isLatest(mine)) void this.refresh();
			else if (revert) this.list = revert();
			return false;
		}
	}
}
