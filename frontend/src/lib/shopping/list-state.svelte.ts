import { ApiError, isSignedOut } from '#lib/api/client.js';
import { getShoppingList, setItemsChecked, type ShoppingList } from '#lib/api/shopping.js';
import { followList } from './live';
import type { Row } from './rows';

/**
 * A shopping list on a page, and the rules for changing it.
 *
 * Changes race: two quick taps send two requests, and their answers may
 * arrive in either order. Every change takes the next local turn and only the
 * newest one's answer replaces the list; an older answer is dropped. So is an
 * answer older than the list shown (a lower server `revision`): a refetch can
 * land before a slow change's answer that it already contains. Answers for
 * another list are dropped too - the page component stays mounted when one
 * list links to the next, so the turn moves on with each load as well.
 */
/** How long a row another device ticked stays in its place, ticked and lit, before it moves to Done. */
export const LINGER_MS = 1500;

export class ListState {
	list = $state<ShoppingList>() as ShoppingList;
	/** Items another device just ticked (seen in a refresh): still shown in their place for a moment. */
	lingering = $state<string[]>([]);
	#rev = 0;
	#lingerTimer: ReturnType<typeof setTimeout> | undefined;

	/** `onfail` is the page's own error handling (toast, redirect when the list is gone). */
	constructor(
		initial: ShoppingList,
		public onfail: (error: unknown) => void
	) {
		this.list = initial;
	}

	/**
	 * Ties the state to its page, called while the page sets up: every later
	 * load replaces the list, and a member's change refetches it. Only the id
	 * reopens the stream: a reload of the same list must not (every open refetches).
	 */
	connect(data: () => ShoppingList) {
		$effect.pre(() => this.load(data()));
		const id = $derived(data().id);
		$effect(() => followList(id, (revision) => void this.follow(revision)));
	}

	/** A new page load: its list replaces the current one and outranks answers in flight. */
	load(next: ShoppingList) {
		this.#rev++;
		this.list = next;
	}

	#current(answer: ShoppingList, mine: number) {
		return (
			mine === this.#rev && answer.id === this.list.id && answer.revision >= this.list.revision
		);
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

	/**
	 * A live event: refetches unless the page shows that revision already, as
	 * after its own change (whose answer carried it). No revision always refetches.
	 */
	follow(revision?: number) {
		if (revision !== undefined && revision <= this.list.revision) return;
		return this.refresh();
	}

	/**
	 * Reloads the list unless a newer change has started meanwhile (its answer
	 * will do). Only a gone list or a lost session reaches `onfail`; any other
	 * failure passes quietly, since the next event or reconnect retries.
	 */
	async refresh() {
		const mine = this.#rev;
		try {
			const answer = await getShoppingList(this.list.id);
			if (this.#current(answer, mine)) this.#take(answer);
		} catch (error) {
			if ((error instanceof ApiError && error.status === 404) || isSignedOut(error))
				this.onfail(error);
		}
	}

	/** A refetched list; what it ticked since lingers in place for a moment. */
	#take(answer: ShoppingList) {
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

	/**
	 * Takes the server's answer as the new list when it is still the newest;
	 * false (and `onfail`, or the caller's own) when the request failed. A failed change undoes its
	 * own optimistic step (`revert`) only while nothing newer was sent;
	 * otherwise the list is reloaded, since undoing locally would also undo
	 * the newer change.
	 */
	async apply(
		request: Promise<ShoppingList>,
		revert?: () => ShoppingList,
		onfail = this.onfail
	): Promise<boolean> {
		const mine = ++this.#rev;
		const id = this.list.id;
		try {
			const answer = await request;
			if (this.#current(answer, mine)) this.list = answer;
			return true;
		} catch (error) {
			onfail(error);
			if (id !== this.list.id) return false;
			if (mine !== this.#rev) void this.refresh();
			else if (revert) this.list = revert();
			return false;
		}
	}
}
