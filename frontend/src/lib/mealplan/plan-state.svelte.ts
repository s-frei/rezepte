import { getPlan, planEventsUrl, type MealPlan } from '#lib/api/mealplan.js';
import { LiveState } from '#lib/live-state.svelte.js';

/**
 * A meal plan's week on a page: `LiveState`'s rules, and an answer for
 * another week is dropped too, since the page stays mounted while the pager
 * moves on and an answer still in flight carries the range it was sent for.
 */
export class PlanState extends LiveState<MealPlan> {
	get plan() {
		return this.value;
	}
	set plan(next: MealPlan) {
		this.value = next;
	}

	/** The shown range, for a request whose answer should carry the same days. */
	get range() {
		return { from: this.plan.from, to: this.plan.to };
	}

	protected fetch() {
		return getPlan(this.plan.id, this.range);
	}
	protected eventsUrl(id: string) {
		return planEventsUrl(id);
	}
	protected key(plan: MealPlan) {
		return `${plan.id}/${plan.from}`;
	}
}
