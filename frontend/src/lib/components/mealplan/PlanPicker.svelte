<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import { Select } from 'bits-ui';
	import type { PlanSummary } from '#lib/api/mealplan.js';
	import { session } from '#lib/auth.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * The plan name ▾ of the week's header: "My plan" first, then the reader's
	 * own named plans and the ones shared with them, in two groups as on the
	 * lists overview. "New plan" joins once plans can be made in the app.
	 */
	let {
		plans,
		value,
		onchange,
		class: className = ''
	}: {
		plans: PlanSummary[];
		value: string;
		onchange: (id: string) => void;
		class?: string;
	} = $props();

	const nameOf = (p: PlanSummary) => (p.personal ? m.plan_personal() : (p.name ?? ''));
	const personal = $derived(plans.filter((p) => p.personal));
	const mine = $derived(plans.filter((p) => !p.personal && p.owner.id === session.user?.id));
	const shared = $derived(plans.filter((p) => !p.personal && p.owner.id !== session.user?.id));
	const groups = $derived(
		[
			{ label: '', plans: personal },
			{ label: m.plan_group_mine(), plans: mine },
			{ label: m.plan_group_shared(), plans: shared }
		].filter((g) => g.plans.length > 0)
	);
	const current = $derived(plans.find((p) => p.id === value));
</script>

<Select.Root type="single" {value} onValueChange={(id) => id !== value && onchange(id)}>
	<Select.Trigger
		aria-label={m.plan_picker()}
		class="inline-flex h-11 max-w-full min-w-0 items-center gap-1.5 rounded-pill border border-border bg-surface hover-tint px-4 text-body-sm font-semibold text-text transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:h-10 {className}"
	>
		<span class="truncate">{current ? nameOf(current) : ''}</span>
		<ChevronDown class="size-3.5 shrink-0" aria-hidden="true" />
	</Select.Trigger>
	<Select.Portal>
		<Select.Content
			preventScroll={false}
			align="start"
			sideOffset={6}
			collisionPadding={16}
			class="z-50 max-w-[calc(100vw-2rem)] min-w-[max(var(--bits-floating-anchor-width),14rem)] origin-(--bits-floating-transform-origin) rounded-2xl border border-popover-border bg-popover p-1.5 shadow-dialog motion-safe:animate-drop motion-reduce:animate-appear"
		>
			{#each groups as group, i (group.label)}
				<Select.Group class={i > 0 ? 'mt-1 border-t border-popover-border pt-1' : ''}>
					{#if group.label}
						<Select.GroupHeading class="px-3 pt-2 pb-1 text-caption font-semibold text-text-muted"
							>{group.label}</Select.GroupHeading
						>
					{/if}
					{#each group.plans as p (p.id)}
						<Select.Item
							value={p.id}
							label={nameOf(p)}
							class="flex min-h-10 cursor-default items-center justify-between gap-6 rounded-sm px-3 py-2 text-body-sm text-text outline-none data-highlighted:bg-popover-highlight"
						>
							{#snippet children({ selected })}
								<span class="min-w-0 break-words">{nameOf(p)}</span>
								{#if selected}
									<Check
										class="size-4.5 shrink-0 text-primary"
										strokeWidth={1.75}
										aria-hidden="true"
									/>
								{/if}
							{/snippet}
						</Select.Item>
					{/each}
				</Select.Group>
			{/each}
		</Select.Content>
	</Select.Portal>
</Select.Root>
