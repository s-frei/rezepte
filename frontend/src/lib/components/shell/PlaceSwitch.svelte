<script lang="ts" module>
	// A pick by the switch itself: the next page's switch takes the focus.
	let handoff = false;
</script>

<script lang="ts">
	import CalendarDays from '@lucide/svelte/icons/calendar-days';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import SegmentedControl from '#lib/components/ui/SegmentedControl.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { setPlaceSide, type PlaceSide } from '#lib/shell.svelte.js';
	import { invitations, invitationsLabel } from '#lib/shopping/invitations.svelte.js';

	/** The side the page shows; the route decides it, not the control. */
	let { side }: { side: PlaceSide } = $props();

	// Arriving on a side by any way - a toast's link, a bookmark - makes it
	// the one the place reopens on.
	$effect(() => setPlaceSide(side));

	// The two sides are two routes, so a pick mounts another page and its
	// own switch, and the focus would drop to the body: an arrow key would
	// end the keyboard's way through the group. The new switch takes the
	// focus onto its checked segment instead; `reset: false` keeps SvelteKit
	// from resetting it after the mount (the switch sits at the page's top,
	// so there is no scroll to reset).
	function pick(next: PlaceSide) {
		setPlaceSide(next);
		handoff = true;
		void goto(resolve(next === 'week' ? '/week' : '/lists'), { reset: false });
	}

	function takeFocus(box: HTMLElement) {
		if (!handoff) return;
		handoff = false;
		box.querySelector<HTMLElement>('[role="radio"][aria-checked="true"]')?.focus();
	}
</script>

<!-- The place's title and its two sides, the head of both pages. Phones put
     the switch under the title at the full width; from `md` it sits at the
     title's end, wide enough for both labels. -->
<div class="mb-6 flex flex-col gap-3 md:flex-row md:items-center md:justify-between md:gap-6">
	<!-- One step smaller below 360px, where "Einkauf & Woche" would break. -->
	<h1
		class="font-display text-display-sm font-medium hyphens-auto min-[360px]:text-display-md md:text-display-lg"
	>
		{m.nav_lists()}
	</h1>
	<div {@attach takeFocus} class="md:w-75">
		<SegmentedControl
			value={side}
			label={m.place_switch()}
			track="surface"
			onchange={pick}
			options={[
				{
					value: 'lists',
					label: m.place_lists(),
					icon: ShoppingBasket,
					dot: invitations.lists > 0 ? invitationsLabel(invitations.lists) : undefined
				},
				{
					value: 'week',
					label: m.place_week(),
					icon: CalendarDays,
					dot: invitations.plans > 0 ? invitationsLabel(invitations.plans) : undefined
				}
			]}
		/>
	</div>
</div>
