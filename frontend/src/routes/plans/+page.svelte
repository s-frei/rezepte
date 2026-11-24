<script lang="ts">
	import CalendarDays from '@lucide/svelte/icons/calendar-days';
	import Plus from '@lucide/svelte/icons/plus';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { afterNavigate, goto, invalidateAll, replaceState } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { redirectSignedOut } from '#lib/api/client.js';
	import { acceptPlan, leavePlan, type WeekPlan } from '#lib/api/mealplan.js';
	import { session } from '#lib/auth.svelte.js';
	import PlanCard from '#lib/components/mealplan/PlanCard.svelte';
	import PlaceSwitch from '#lib/components/shell/PlaceSwitch.svelte';
	import InvitationCard from '#lib/components/shopping/InvitationCard.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import { holdKeyboard } from '#lib/keyboard.js';
	import { planHref } from '#lib/mealplan/href.js';
	import { arrival, byUse } from '#lib/mealplan/overview.svelte.js';
	import { mondayOf, weekDays } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { rememberPlan, shell } from '#lib/shell.svelte.js';
	import { invitations } from '#lib/shopping/invitations.svelte.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// A 401 in the load: redirected here, where the address is this page.
	$effect(() => {
		if (data.signedOut) void redirectSignedOut();
	});

	// Each section: a card per plan, the ones in use first. A plan empty
	// this week stays a card, so it is found where the others are.
	const sorted = (plans: WeekPlan[]) => [...plans].sort(byUse);
	const mine = $derived(sorted(data.plans.filter((p) => p.owner.id === session.user?.id)));
	const shared = $derived(sorted(data.plans.filter((p) => p.owner.id !== session.user?.id)));
	// Only "My plan": a light nudge to plan together.
	const alone = $derived(data.plans.length === 1 && data.invitations.length === 0);

	// Answered here: gone at once, before the next read. Each read starts
	// afresh: the server knows the answers then, and a new invitation to
	// the same list or plan counts again.
	let answered = $derived.by(() => {
		void data;
		return [] as string[];
	});
	const invited = $derived(data.invitations.filter((i) => !answered.includes(i.planId)));
	$effect(() => {
		invitations.plans = invited.length;
	});
	$effect(() => {
		if (invitations.plans !== untrack(() => invited.length)) void invalidateAll();
	});

	/** When the overview last read its plans again, to take one burst of events as one. */
	let read = 0;

	// A plan that was gone for the reader sent them here (`?gone`): said once,
	// and the address drops it, so a reload or Back does not say it again.
	// Sent from this overview (a stale card), the plan's link added an entry:
	// back to the one it came from, so Back is not a step to the same page.
	// After the navigation: the router takes no `replaceState` before it is up.
	let stepped = false;
	afterNavigate(({ from }) => {
		if (!page.url.searchParams.has('gone')) {
			// Back on the entry before: the load does not read `gone`, so it
			// kept the card that led away.
			if (stepped) {
				stepped = false;
				read = Date.now();
				void invalidateAll();
			}
			return;
		}
		toast.error(m.plan_gone());
		if (from?.route.id === '/plans') {
			stepped = true;
			history.back();
			return;
		}
		const url = new URL(page.url.href);
		url.searchParams.delete('gone');
		replaceState(url, page.state);
		read = Date.now();
		void invalidateAll();
	});

	// Back from a plan that was deleted or left: the focus where the toast talks.
	$effect(() => {
		if (!arrival.overview) return;
		arrival.overview = false;
		document.querySelector<HTMLElement>('[data-plans-heading]')?.focus();
	});

	// Nothing streams the overview: it reads again whenever the reader comes
	// back to it (tab, window, the back-forward cache), which also moves it
	// on to a new day or week.
	function reread() {
		if (Date.now() - read < 2000) return;
		read = Date.now();
		void invalidateAll();
	}

	async function accept(planId: string) {
		const monday = mondayOf(data.today);
		await acceptPlan(planId, { from: monday, to: weekDays(monday)[6] });
		rememberPlan(planId);
		// The plan opens in the overview's place: its count drops here, as the card goes.
		answered = [...answered, planId];
		invitations.plans = Math.max(0, invitations.plans - 1);
		await goto(planHref(planId));
	}

	function newPlan() {
		// Inside the tap: the draft's name field takes the keyboard over.
		holdKeyboard();
		void goto(resolve('/plans/new'));
	}

	// The overview makes plans: "New plan" in the top bar and behind a phone's "+".
	$effect(() => {
		shell.actions = newAction;
		shell.add = { label: m.plan_new(), open: newPlan, expanded: false };
		return () => {
			shell.actions = undefined;
			shell.add = undefined;
		};
	});
</script>

{#snippet newAction()}
	<Button variant="primary" onclick={newPlan}>
		<Plus class="size-4" aria-hidden="true" />
		{m.plan_new()}
	</Button>
{/snippet}

<svelte:head><title>{m.plans_title()} · {m.app_name()}</title></svelte:head>

<svelte:window onfocus={reread} onpageshow={(event) => event.persisted && reread()} />
<svelte:document onvisibilitychange={() => !document.hidden && reread()} />

<div class="pt-6 pb-8 md:pt-4">
	<PlaceSwitch side="week" />

	{#if data.signedOut}
		<!-- Leaving for the sign-in. -->
	{:else if invited.length > 0}
		<h2 class="sr-only">{m.invitations_heading()}</h2>
		<div class="flex flex-col gap-3">
			{#each invited as invitation (invitation.planId)}
				<InvitationCard
					from={invitation.inviter}
					title={m.plan_invite_title({
						name: invitation.inviter.displayName,
						plan: invitation.name ?? ''
					})}
					hint={m.plan_invite_hint()}
					gone={m.plan_gone()}
					accept={() => accept(invitation.planId)}
					decline={() => leavePlan(invitation.planId)}
					ondone={() => (answered = [...answered, invitation.planId])}
				/>
			{/each}
		</div>
	{/if}

	{#each [{ title: m.plans_mine(), plans: mine }, { title: m.plan_group_shared(), plans: shared }] as section, s (section.title)}
		{@const count = section.plans.length}
		{#if count > 0}
			<div
				class="{s === 0 && invited.length === 0 ? 'mt-2' : 'mt-8'} mb-3.5 flex items-center gap-3"
			>
				<h2
					class="font-display text-heading font-medium outline-none"
					tabindex="-1"
					data-plans-heading={s === 0 ? '' : undefined}
				>
					{section.title}<span class="sr-only"> ({m.plans_section_count({ count })})</span>
				</h2>
				<span aria-hidden="true" class="text-caption font-semibold text-text-muted">{count}</span>
				<span aria-hidden="true" class="min-w-6 flex-1 border-b-2 border-dotted border-handle"
				></span>
			</div>
			{#if section.plans.length > 0 || (s === 0 && alone)}
				<!-- The lists' grid; each card's five parts are rows of it (subgrid), so
				     strips, today lines and feet line up across a row and no card
				     stretches into an empty block. -->
				<div class="grid grid-cols-[repeat(auto-fill,minmax(min(100%,320px),1fr))] gap-4">
					{#each section.plans as plan (plan.id)}
						<PlanCard {plan} today={data.today} />
					{/each}
					{#if s === 0 && alone}
						<!-- The nudge sits where the next plan's card would. -->
						<div
							class="row-span-4 flex items-start gap-3.5 self-start rounded-2xl border-[1.5px] border-dashed border-handle px-4 py-4 min-[360px]:px-5"
						>
							<span
								class="flex size-10 shrink-0 items-center justify-center rounded-md bg-accent text-accent-foreground"
								aria-hidden="true"><CalendarDays class="size-5" /></span
							>
							<div class="flex min-w-0 flex-col items-start gap-3">
								<div>
									<h3 class="font-display text-card font-medium">{m.plans_hint_title()}</h3>
									<p class="mt-1 text-body-sm text-text-muted">{m.plans_hint_text()}</p>
								</div>
								<Button variant="secondary" onclick={newPlan} class="max-md:h-11">
									<Plus class="size-4" aria-hidden="true" />
									{m.plan_new()}
								</Button>
							</div>
						</div>
					{/if}
				</div>
			{/if}
		{/if}
	{/each}
</div>
