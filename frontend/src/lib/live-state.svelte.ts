import { ApiError, isSignedOut } from '#lib/api/client.js';
import { follow } from '#lib/live.js';

/**
 * A shared thing on a page - a shopping list, a meal plan's week - and the
 * rules for changing it.
 *
 * Changes race: two quick taps send two requests, and their answers may
 * arrive in either order. Every change takes the next local turn and only the
 * newest one's answer replaces the value; an older answer is dropped. So is an
 * answer older than the value shown (a lower server `revision`): a refetch can
 * land before a slow change's answer that it already contains. Answers for
 * another `key` (another list or plan, another week) are dropped too - the page
 * stays mounted when one links to the next or the pager moves on, so the turn
 * moves on with each load as well.
 */
export abstract class LiveState<T extends { id: string; revision: number }> {
	value = $state<T>() as T;
	#rev = 0;

	/** `onfail` is the page's own error handling (toast, redirect when the thing is gone). */
	constructor(
		initial: T,
		public onfail: (error: unknown) => void
	) {
		this.value = initial;
	}

	/** Fetches the value as shown now. */
	protected abstract fetch(): Promise<T>;
	/** The events stream of the thing with `id`. */
	protected abstract eventsUrl(id: string): string;
	/** What an answer must share with the shown value to replace it. */
	protected key(value: T): string {
		return value.id;
	}
	/** A refetched value replaces the shown one. */
	protected take(answer: T) {
		this.value = answer;
	}

	/**
	 * Ties the state to its page, called while the page sets up: every later
	 * load replaces the value, and a member's change refetches it. Only the id
	 * reopens the stream: a reload of the same thing (or another week of it)
	 * must not.
	 */
	connect(data: () => T) {
		$effect.pre(() => this.load(data()));
		const id = $derived(data().id);
		$effect(() => follow(this.eventsUrl(id), (revision) => void this.follow(revision)));
	}

	/** A new page load: its value replaces the current one and outranks answers in flight. */
	load(next: T) {
		this.#rev++;
		this.value = next;
	}

	#current(answer: T, mine: number) {
		return (
			mine === this.#rev &&
			this.key(answer) === this.key(this.value) &&
			answer.revision >= this.value.revision
		);
	}

	/**
	 * A live event: refetches unless the page shows that revision already, as
	 * after its own change (whose answer carried it). No revision always refetches.
	 */
	follow(revision?: number) {
		if (revision !== undefined && revision <= this.value.revision) return;
		return this.refresh();
	}

	/**
	 * Refetches unless a newer change has started meanwhile (its answer will
	 * do). Only a gone thing or a lost session reaches `onfail`; any other
	 * failure passes quietly, since the next event or reconnect retries.
	 */
	async refresh() {
		const mine = this.#rev;
		try {
			const answer = await this.fetch();
			if (this.#current(answer, mine)) this.take(answer);
		} catch (error) {
			if ((error instanceof ApiError && error.status === 404) || isSignedOut(error))
				this.onfail(error);
		}
	}

	/**
	 * Takes the server's answer as the new value when it is still the newest;
	 * false (and `onfail`, or the caller's own) when the request failed. A
	 * failed change undoes its own optimistic step (`revert`) only while
	 * nothing newer was sent; otherwise the value is refetched, since undoing
	 * locally would also undo the newer change.
	 */
	async apply(request: Promise<T>, revert?: () => T, onfail = this.onfail): Promise<boolean> {
		const mine = ++this.#rev;
		const key = this.key(this.value);
		try {
			const answer = await request;
			if (this.#current(answer, mine)) this.value = answer;
			return true;
		} catch (error) {
			onfail(error);
			if (key !== this.key(this.value)) return false;
			if (mine !== this.#rev) void this.refresh();
			else if (revert) this.value = revert();
			return false;
		}
	}
}
