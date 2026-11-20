<script lang="ts">
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import { Dialog } from 'bits-ui';
	import type { PlanEntry } from '#lib/api/mealplan.js';
	import IconButton from '#lib/components/ui/IconButton.svelte';
	import ResponsiveDialog from '#lib/components/ui/ResponsiveDialog.svelte';
	import { dayNames, shiftWeek, weekDays, weekRange } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';

	/**
	 * "Move to a day…": the keyboard's and a screen reader's way to move an
	 * entry, and a phone's quick one. The shown week's days and "Someday",
	 * one tap each; the pager reaches the weeks around it (a meal pushed to
	 * next Monday). The entry's own day is marked and cannot be picked.
	 */
	let {
		open = $bindable(false),
		entry,
		monday,
		today,
		onpick,
		onCloseAutoFocus
	}: {
		open?: boolean;
		entry: PlanEntry | null;
		/** The shown week's Monday, where the sheet starts. */
		monday: string;
		today: string;
		onpick: (entry: PlanEntry, day: string | null) => void;
		/** Where focus goes once the sheet has closed; the week sends it to the moved card. */
		onCloseAutoFocus?: (event: Event) => void;
	} = $props();

	const locale = getLocale();
	let week = $state('');
	$effect(() => {
		if (open) week = monday;
	});
	const days = $derived(week ? weekDays(week) : []);

	function pick(day: string | null) {
		if (!entry) return;
		open = false;
		onpick(entry, day);
	}

	const option =
		'flex min-h-13 w-full flex-col items-start justify-center rounded-lg border border-border bg-surface-elevated px-3.5 py-1.5 text-left transition hover:border-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:border-primary disabled:bg-accent disabled:text-accent-foreground';
</script>

{#snippet body()}
	<Dialog.Title class="font-display text-heading font-medium">{m.plan_move_title()}</Dialog.Title>
	{#if entry}
		<Dialog.Description class="mt-1 text-body-sm [hyphens:none] text-text-muted"
			>{entry.title}</Dialog.Description
		>
	{/if}
	<div class="mt-4 flex items-center justify-between gap-2">
		<IconButton
			label={m.week_previous()}
			class="max-md:size-11"
			onclick={() => (week = shiftWeek(week, -1))}
		>
			<ChevronLeft class="size-4" aria-hidden="true" />
		</IconButton>
		<p class="min-w-0 text-center font-display text-card-sm font-medium" aria-live="polite">
			{week ? weekRange(week, locale, 'short') : ''}
		</p>
		<IconButton
			label={m.week_next()}
			class="max-md:size-11"
			onclick={() => (week = shiftWeek(week, 1))}
		>
			<ChevronRight class="size-4" aria-hidden="true" />
		</IconButton>
	</div>
	<ul class="mt-3 grid grid-cols-2 gap-2">
		{#each days as day (day)}
			{@const names = dayNames(day, locale)}
			{@const here = entry?.day === day}
			<li>
				<button
					type="button"
					aria-label={day === today ? `${names.full}, ${m.week_today()}` : names.full}
					aria-current={here ? 'true' : undefined}
					disabled={here}
					onclick={() => pick(day)}
					class={option}
				>
					<span class="text-body-sm font-semibold">{names.long}</span>
					<span class="text-caption opacity-80"
						>{names.date}{day === today ? ` · ${m.week_today()}` : ''}</span
					>
				</button>
			</li>
		{/each}
		<li>
			<button
				type="button"
				aria-label={m.plan_tray()}
				aria-current={entry?.day === null ? 'true' : undefined}
				disabled={entry?.day === null}
				onclick={() => pick(null)}
				class={option}
			>
				<span class="text-body-sm font-semibold">{m.plan_tray()}</span>
				<span class="text-caption opacity-80">{m.plan_tray_empty()}</span>
			</button>
		</li>
	</ul>
{/snippet}

<ResponsiveDialog
	bind:open
	closeLabel={m.common_cancel()}
	bodyClass="pb-6"
	{onCloseAutoFocus}
	{body}
/>
