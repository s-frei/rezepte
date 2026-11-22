<script lang="ts">
	import House from '@lucide/svelte/icons/house';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import { Dialog, RadioGroup } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { untrack } from 'svelte';
	import { isSignedOut } from '#lib/api/client.js';
	import { getStaples, setStaple } from '#lib/api/mealplan.js';
	import type { Department, PlanRef } from '#lib/api/shopping.js';
	import ResponsiveDialog from '#lib/components/ui/ResponsiveDialog.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Switch from '#lib/components/ui/Switch.svelte';
	import { DEPARTMENTS, departmentLabel } from '#lib/shopping/departments.js';
	import type { Row } from '#lib/shopping/rows.js';
	import { m } from '#lib/paraglide/messages.js';
	import { sourceOrigin } from './ShoppingRow.svelte';

	let {
		open = $bindable(false),
		row,
		plans,
		department,
		onpick,
		onremove
	}: {
		open?: boolean;
		/** The row last tapped; kept while the sheet closes. */
		row: Row | null;
		/** The plans the reader is in that the name came from (`stapleOrigins`). */
		plans: PlanRef[];
		department: Department;
		/** Moves the name to another department for the whole instance. */
		onpick: (department: Department) => void;
		/** Takes every item behind the row off the list. */
		onremove: () => void;
	} = $props();

	// "Haben wir immer" for each plan the name came from, one switch each,
	// acting on that plan alone. Read when the sheet opens; a plan's state is
	// null until known (or when the answer failed), and its switch waits.
	let staple = $state<Record<string, boolean | null>>({});
	let stapling = $state('');
	let req = 0;
	$effect(() => {
		if (!open || !row) return;
		const key = row.key;
		const ids = plans.map((p) => p.planId!);
		untrack(() => void readStaples(key, ids));
	});

	async function readStaples(key: string, ids: string[]) {
		const mine = ++req;
		staple = Object.fromEntries(ids.map((id) => [id, null]));
		await Promise.all(
			ids.map(async (id) => {
				try {
					const { staples } = await getStaples(id);
					if (mine === req) staple[id] = staples.some((s) => s.nameKey === key);
				} catch {
					// Left unknown: that switch stays disabled, the rest of the sheet works.
				}
			})
		);
	}

	async function toggleStaple(planId: string, on: boolean) {
		if (!row || stapling) return;
		const { name, key } = row;
		const was = staple[planId];
		staple[planId] = on;
		stapling = planId;
		try {
			const { staples } = await setStaple(planId, name, on);
			staple[planId] = staples.some((s) => s.nameKey === key);
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.item_sheet_staple_error());
			staple[planId] = was;
		} finally {
			stapling = '';
		}
	}

	const planName = (p: PlanRef) => (p.planPersonal ? m.plan_personal() : (p.planName ?? ''));
	const any = $derived(Object.values(staple).some(Boolean));
</script>

{#snippet body()}
	{#if row}
		<Dialog.Title class="font-display text-heading-lg font-medium text-pretty"
			>{row.name}</Dialog.Title
		>
		<p class="mt-1 flex flex-wrap gap-x-3 text-caption text-text-muted">
			{#each row.sources as source (source.itemId)}
				{@const origin = sourceOrigin(source)}
				{#if source.amount || origin}
					<span
						>{#if source.amount}<b class="font-bold text-primary">{source.amount}</b
							>{/if}{#if source.amount && origin}&nbsp;·&nbsp;{/if}{origin ?? ''}</span
					>
				{/if}
			{/each}
		</p>
		{#if plans.length > 0}
			<!-- On, the name starts unpicked whenever that plan's recipes go on a list; nothing leaves this list.
			     One switch per plan, each for its plan alone; a name from no plan of the reader's has none. -->
			<div
				class="mt-5 flex items-start gap-3 rounded-2xl border border-border bg-surface-elevated px-4 py-3"
			>
				<House
					class="mt-0.5 size-4.5 shrink-0 {any ? 'text-primary' : 'text-text-muted'}"
					fill={any ? 'currentColor' : 'none'}
					fill-opacity={any ? 0.18 : 0}
					aria-hidden="true"
				/>
				<div class="min-w-0 flex-1">
					{#snippet toggle(plan: PlanRef)}
						<Switch
							checked={staple[plan.planId!] ?? false}
							disabled={staple[plan.planId!] == null || stapling !== ''}
							label={m.plan_staple_toggle_in({ name: row.name, plan: planName(plan) })}
							describedby="item-sheet-staple-hint"
							onchange={(on) => void toggleStaple(plan.planId!, on)}
						/>
					{/snippet}
					{#if plans.length === 1}
						{@const plan = plans[0]}
						<div class="flex items-start gap-3">
							<!-- The label and "· Haushalt" each stay whole: where both do not fit,
							     the plan moves to the next line, and only a name longer than a line is cut. -->
							<span class="min-w-0 flex-1 text-body-sm font-medium">
								<span class="whitespace-nowrap">{m.plan_staple()}</span>
								<span
									class="inline-block max-w-full truncate align-bottom font-normal whitespace-nowrap text-text-muted"
									>·&nbsp;{planName(plan)}</span
								>
							</span>
							{@render toggle(plan)}
						</div>
					{:else}
						<span class="block text-body-sm font-medium">{m.plan_staple()}</span>
						<ul class="mt-1">
							{#each plans as plan (plan.planId)}
								<li
									class="flex min-h-11 items-center gap-3 border-b border-dashed border-border last:border-b-0"
								>
									<span class="min-w-0 flex-1 py-2 text-body-sm break-words">{planName(plan)}</span>
									{@render toggle(plan)}
								</li>
							{/each}
						</ul>
					{/if}
					<p id="item-sheet-staple-hint" class="mt-0.5 text-caption text-text-muted">
						{plans.length === 1 ? m.item_sheet_staple_hint() : m.item_sheet_staple_hint_each()}
					</p>
				</div>
			</div>
		{/if}
		<p
			id="item-sheet-department"
			class="mt-6 mb-2 text-label font-bold tracking-wide text-text-muted uppercase"
		>
			{m.item_sheet_department()}
		</p>
		<RadioGroup.Root
			value={department}
			onValueChange={(value) => onpick(value as Department)}
			aria-labelledby="item-sheet-department"
			class="rounded-2xl border border-border bg-surface-elevated px-4"
		>
			{#each DEPARTMENTS as d (d)}
				<RadioGroup.Item
					value={d}
					class="flex w-full items-center gap-3 border-b border-dashed border-border py-2.5 text-left text-body last:border-b-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary data-[state=checked]:font-semibold"
				>
					{#snippet children({ checked })}
						<span
							aria-hidden="true"
							class="size-5 shrink-0 rounded-full {checked
								? 'border-[6px] border-primary'
								: 'border-[1.5px] border-text-muted'}"
						></span>
						{departmentLabel(d)}
					{/snippet}
				</RadioGroup.Item>
			{/each}
		</RadioGroup.Root>
		<p class="mt-3 text-caption text-text-muted">{m.item_sheet_remembered({ name: row.name })}</p>
	{/if}
{/snippet}

{#snippet footer()}
	<!-- One line where both fit; on the narrowest phones "Fertig" wraps below, never cutting the label. -->
	<!-- Button's secondary look in the destructive ink (its own text-text would compete). -->
	<button
		type="button"
		onclick={onremove}
		class="inline-flex h-10 grow items-center justify-center gap-2 rounded-pill border border-border bg-surface hover-tint px-4 text-body-sm font-semibold whitespace-nowrap text-destructive transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] max-md:h-11 md:grow-0"
	>
		<Trash2 class="size-4 shrink-0" aria-hidden="true" />
		{m.item_sheet_remove()}
	</button>
	<Button onclick={() => (open = false)} class="grow max-md:h-11 md:grow-0"
		>{m.item_sheet_done()}</Button
	>
{/snippet}

<ResponsiveDialog bind:open closeLabel={m.item_sheet_done()} {body} {footer} />
