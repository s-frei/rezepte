<script lang="ts">
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Plus from '@lucide/svelte/icons/plus';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { ApiError, isSignedOut } from '#lib/api/client.js';
	import type { PlanSummary } from '#lib/api/mealplan.js';
	import PlaceSwitch from '#lib/components/shell/PlaceSwitch.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import IconButton from '#lib/components/ui/IconButton.svelte';
	import NoteEmptyState from '#lib/components/ui/NoteEmptyState.svelte';
	import Select from '#lib/components/ui/Select.svelte';
	import { PlanState } from '#lib/mealplan/plan-state.svelte.js';
	import { mondayOf, shiftWeek, weekRange } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { lastPlan, rememberPlan, shell } from '#lib/shell.svelte.js';
	import planArt from '$brand/lockups/rezepte-mascot-shopping.webp?url';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// The plan went away under the page (deleted, or the reader removed):
	// the week reopens on the personal plan.
	function fail(error: unknown) {
		if (isSignedOut(error)) return;
		if (error instanceof ApiError && error.status === 404) {
			toast.error(m.week_load_error());
			void goto(resolve('/week'));
			return;
		}
		toast.error(m.week_load_error());
	}

	// Starts as the first load; `connect` follows each later one (another
	// week, another plan), and a member's change refetches the shown week.
	// svelte-ignore state_referenced_locally
	const week = new PlanState(data.plan, fail);
	week.connect(() => data.plan);
	const plan = $derived(week.plan);

	const locale = getLocale();
	const monday = $derived(plan.from);
	const thisWeek = $derived(mondayOf(data.today));
	const empty = $derived(plan.entries.length === 0);

	const planName = (p: Pick<PlanSummary, 'personal' | 'name'>) =>
		p.personal ? m.plan_personal() : (p.name ?? '');

	$effect(() => rememberPlan(plan.id));

	/**
	 * Another week or plan. `plan` is left out only for the plan the page
	 * would reopen on anyway, the one remembered last (the shown one); a pick
	 * of another plan always names it, the personal one too.
	 */
	function show(from: string, id = plan.id) {
		const query: [string, string][] = [];
		if (id !== lastPlan()) query.push(['plan', id]);
		if (from !== thisWeek) query.push(['from', from]);
		const qs = new URLSearchParams(query).toString();
		void goto(resolve(qs ? `/week?${qs}` : '/week'), { reset: false });
	}

	// The week's action takes the place of New list in the top bar; it opens
	// the sheet that plans a recipe or a note.
	let adding = $state(false);
	$effect(() => {
		shell.actions = addAction;
		return () => (shell.actions = undefined);
	});
</script>

{#snippet addAction()}
	<Button
		variant="primary"
		aria-haspopup="dialog"
		aria-expanded={adding}
		onclick={() => (adding = true)}
	>
		<Plus class="size-4" aria-hidden="true" />
		{m.week_add()}
	</Button>
{/snippet}

<svelte:head><title>{weekRange(monday, locale)} · {m.app_name()}</title></svelte:head>

<div class="pt-6 pb-8 md:pt-4">
	<PlaceSwitch side="week" />

	<div class="mb-6 flex flex-wrap items-center gap-x-4 gap-y-3 max-md:justify-between">
		<h2 class="font-display text-card font-medium md:text-heading-lg">
			<span class="max-md:hidden">{weekRange(monday, locale)}</span>
			<span class="md:hidden">{weekRange(monday, locale, 'short')}</span>
		</h2>
		<div class="flex items-center gap-1.5">
			<IconButton
				label={m.week_previous()}
				class="max-md:size-11"
				onclick={() => show(shiftWeek(monday, -1))}
			>
				<ChevronLeft class="size-4" aria-hidden="true" />
			</IconButton>
			<Button variant="secondary" class="max-md:h-11" onclick={() => show(thisWeek)}>
				{m.week_today()}
			</Button>
			<IconButton
				label={m.week_next()}
				class="max-md:size-11"
				onclick={() => show(shiftWeek(monday, 1))}
			>
				<ChevronRight class="size-4" aria-hidden="true" />
			</IconButton>
		</div>
		{#if data.plans.length > 1}
			<Select
				label={m.plan_picker()}
				value={plan.id}
				options={data.plans.map((p) => ({ value: p.id, label: planName(p) }))}
				onchange={(id) => show(monday, id)}
				class="bg-surface md:ml-auto"
			/>
		{/if}
	</div>

	{#if empty}
		<NoteEmptyState
			art={planArt}
			title={m.week_empty_title()}
			steps={[
				[m.week_empty_step_plan(), m.week_empty_step_plan_hint()],
				[m.week_empty_step_list(), m.week_empty_step_list_hint()],
				[m.week_empty_step_shop(), m.week_empty_step_shop_hint()]
			]}
		>
			<Button
				variant="primary"
				size="lg"
				onclick={() => (adding = true)}
				class="group shadow-cta md:h-12"
			>
				<Plus
					aria-hidden="true"
					strokeWidth={2.5}
					class="size-5 transition-transform duration-300 ease-spring group-hover:rotate-90 motion-reduce:transition-none"
				/>
				{m.week_add()}
			</Button>
		</NoteEmptyState>
	{/if}
</div>
