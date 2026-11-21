import { ApiError, isSignedOut } from '#lib/api/client.js';
import { getPlan, planEventsUrl, type MealPlan } from '#lib/api/mealplan.js';
import { follow } from '#lib/live.js';

/**
 * A meal plan's week on a page, and the rules for changing it - the same as
 * `ListState`'s for a list. Every change takes the next local turn and only
 * the newest one's answer replaces the plan; an older answer is dropped, as
 * is one with a lower server `revision`, one for another plan, or one for
 * another week: the page stays mounted while the pager moves on, or a link
 * opens another plan, and an answer still in flight carries the range it was
 * sent for.
 */
export class PlanState {
	plan = $state<MealPlan>() as MealPlan;
	#rev = 0;

	/** `onfail` is the page's own error handling (toast, redirect when the plan is gone). */
	constructor(
		initial: MealPlan,
		public onfail: (error: unknown) => void
	) {
		this.plan = initial;
	}

	/**
	 * Ties the state to its page, called while the page sets up: every later
	 * load (another week, another plan) replaces the plan, and a member's
	 * change refetches it. Only the plan id reopens the stream: paging weeks
	 * keeps it.
	 */
	connect(data: () => MealPlan) {
		$effect.pre(() => this.load(data()));
		const id = $derived(data().id);
		$effect(() => follow(planEventsUrl(id), (revision) => void this.follow(revision)));
	}

	/** A new page load: its plan replaces the current one and outranks answers in flight. */
	load(next: MealPlan) {
		this.#rev++;
		this.plan = next;
	}

	/** The shown range, for a request whose answer should carry the same days. */
	get range() {
		return { from: this.plan.from, to: this.plan.to };
	}

	#current(answer: MealPlan, mine: number) {
		return (
			mine === this.#rev &&
			answer.id === this.plan.id &&
			answer.from === this.plan.from &&
			answer.revision >= this.plan.revision
		);
	}

	/**
	 * A live event: refetches unless the page shows that revision already, as
	 * after its own change. No revision always refetches.
	 */
	follow(revision?: number) {
		if (revision !== undefined && revision <= this.plan.revision) return;
		return this.refresh();
	}

	/**
	 * Reloads the shown week unless a newer change has started meanwhile.
	 * Only a gone plan or a lost session reaches `onfail`; anything else
	 * passes quietly, since the next event or reconnect retries.
	 */
	async refresh() {
		const mine = this.#rev;
		try {
			const answer = await getPlan(this.plan.id, this.range);
			if (this.#current(answer, mine)) this.plan = answer;
		} catch (error) {
			if ((error instanceof ApiError && error.status === 404) || isSignedOut(error))
				this.onfail(error);
		}
	}

	/**
	 * Takes the server's answer as the new plan when it is still the newest;
	 * false (and `onfail`, or the caller's own) when the request failed. A
	 * failed change undoes its own optimistic step (`revert`) only while
	 * nothing newer was sent; otherwise the week is reloaded.
	 */
	async apply(
		request: Promise<MealPlan>,
		revert?: () => MealPlan,
		onfail = this.onfail
	): Promise<boolean> {
		const mine = ++this.#rev;
		const { id, from } = this.plan;
		try {
			const answer = await request;
			if (this.#current(answer, mine)) this.plan = answer;
			return true;
		} catch (error) {
			onfail(error);
			if (id !== this.plan.id || from !== this.plan.from) return false;
			if (mine !== this.#rev) void this.refresh();
			else if (revert) this.plan = revert();
			return false;
		}
	}
}
