<script lang="ts">
	import { flip } from 'svelte/animate';
	import { prefersReducedMotion } from 'svelte/motion';
	import { tick } from 'svelte';
	import Check from '@lucide/svelte/icons/check';
	import GripVertical from '@lucide/svelte/icons/grip-vertical';
	import Plus from '@lucide/svelte/icons/plus';
	import X from '@lucide/svelte/icons/x';
	import { dragHandle, dragHandleZone, type DndEvent } from 'svelte-dnd-action';
	import Button from '#lib/components/ui/Button.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { newStep, type FieldErrors, type FormGroup, type FormStep } from '#lib/recipe/form.js';
	import {
		acceptAll,
		countPending,
		waitingChoices,
		pickerEntries
	} from '#lib/recipe/step-references.js';
	import { ACCEPT, ACTION, LIST, ROW } from './link-list';
	import StepField from './StepField.svelte';
	import { collapse, fresh, reveal } from './motion';

	let {
		steps = $bindable([]),
		groups,
		errors = {}
	}: {
		steps?: FormStep[];
		/** The recipe's ingredient groups; every link points into one of them. */
		groups: FormGroup[];
		errors?: FieldErrors;
	} = $props();

	const FLIP_DURATION = $derived(prefersReducedMotion.current ? 0 : 150);

	const entries = $derived(pickerEntries(groups));
	const pendingCount = $derived(countPending(steps, groups));
	/** Words naming an ingredient in several groups that no save will link. */
	const waiting = $derived(waitingChoices(steps, groups));

	async function addStep(index = steps.length) {
		const step = newStep();
		fresh.add(step.id);
		steps.splice(index, 0, step);
		await tick();
		document.getElementById(`step-${step.id}`)?.focus();
		fresh.delete(step.id);
	}

	/**
	 * Removing a step destroys the x button that had focus, so focus would
	 * fall to `<body>`. Hand it to the step that takes its place: the one
	 * above, or - when the first step goes - the one that moves up into index
	 * 0, which is the fresh blank step when the list was emptied. A step
	 * among others folds shut first; it is looked up again afterwards, since
	 * the list can have changed while it closed.
	 */
	async function removeStep(index: number) {
		const step = steps[index];
		if (steps.length > 1) {
			const neighbor = steps[index === 0 ? 1 : index - 1];
			document.getElementById(`step-${neighbor.id}`)?.focus();
			await collapse(document.getElementById(`step-${step.id}`)?.closest('li'));
		}
		const current = steps.findIndex((s) => s.id === step.id);
		if (current === -1) return;
		steps.splice(current, 1);
		if (steps.length === 0) {
			steps.push(newStep());
			await tick();
			document.getElementById(`step-${steps[0].id}`)?.focus();
		}
	}

	function reorder(event: CustomEvent<DndEvent<FormStep>>) {
		steps = event.detail.items;
	}

	/** What "Accept all" did, for a screen reader. */
	let said = $state('');

	/**
	 * Takes every open proposal and says how many. The button goes with them,
	 * so focus moves to the box it was in while that stays for a waiting
	 * word, else to the first step's list toggle - not into a step, which
	 * would raise a phone's keyboard and open the card of a word at its start.
	 */
	function acceptEverything() {
		const count = acceptAll(steps, groups);
		said = m.editor_reference_accepted_all({ count });
		void tick().then(() =>
			(
				document.getElementById('accept-all-box') ??
				document.getElementById(`step-links-toggle-${steps[0]?.id}`)
			)?.focus()
		);
	}
</script>

<div class="space-y-3">
	<p role="status" class="sr-only">{said}</p>
	{#if pendingCount > 0 || waiting.length > 0}
		<!--
			The one place the matcher's work is visible as a whole, drawn as a
			row of the lists under the steps: the same surface, the same filled
			circle that takes one proposal there takes them all here. From `md:`
			up it repeats the step rows' grid, so it spans the fields' column and
			its circle heads the column of circles below it. Only its circle is
			the button, as on every row there; a save takes no proposal, so this
			is the one way to take them all at once.
		-->
		<div class="md:grid md:grid-cols-[20px_32px_1fr_28px] md:gap-x-2">
			<div id="accept-all-box" tabindex="-1" class="{LIST} outline-none md:col-start-3">
				{#if pendingCount > 0}
					<div class={ROW}>
						<span class="flex min-w-0 flex-col">
							<span class="font-medium text-primary">{m.editor_reference_accept_all()}</span>
							<span id="accept-all-count" class="text-caption text-text-muted">
								{pendingCount === 1
									? m.editor_reference_open_one()
									: m.editor_reference_open({ count: pendingCount })}
							</span>
						</span>
						<!-- Only the circle takes, as in the lists below: the row is its label. -->
						<button
							type="button"
							onclick={acceptEverything}
							aria-label={m.editor_reference_accept_all()}
							aria-describedby="accept-all-count"
							class="{ACTION} {ACCEPT}"
						>
							<Check class="size-4" aria-hidden="true" />
						</button>
					</div>
				{/if}
				<!-- A save leaves these unlinked, so the note says they wait rather than counting them in. -->
				{#if waiting.length > 0}
					<p class="px-3.5 py-2 text-caption text-text-muted md:py-2.5">
						{waiting.length === 1
							? m.editor_reference_waiting_one(waiting[0])
							: m.editor_reference_waiting({ count: waiting.length })}
					</p>
				{/if}
			</div>
		</div>
	{/if}

	<!-- The `aria-label`s on the zone and on every item are what
	     svelte-dnd-action reads out while dragging (see its README); without
	     them its announcements say "item" and "list". -->
	<ul
		use:dragHandleZone={{
			items: steps,
			type: 'step',
			flipDurationMs: FLIP_DURATION,
			dropTargetStyle: {}
		}}
		onconsider={reorder}
		onfinalize={reorder}
		aria-label={m.editor_section_steps()}
		class="space-y-2"
	>
		{#each steps as step, index (step.id)}
			<!--
				On a phone the handle, the number and the remove button take a row
				of their own above the field, which then spans the full width: in
				one row with them, the sentence got half the screen and broke after
				every second word. From `md:` up they sit beside it again.
			-->
			<li
				animate:flip={{ duration: fresh.size > 0 ? 0 : FLIP_DURATION }}
				in:reveal={{ fresh: fresh.has(step.id) }}
				aria-label={m.editor_step_number({ number: index + 1 })}
				class="grid list-none grid-cols-[20px_32px_1fr_28px] items-center gap-x-2 gap-y-1.5 md:items-start md:gap-y-2"
			>
				<button
					use:dragHandle
					type="button"
					aria-label={m.editor_step_reorder()}
					class="row-start-1 flex cursor-grab items-center justify-center text-handle focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:mt-4"
				>
					<GripVertical class="size-4" aria-hidden="true" />
				</button>
				<span
					aria-hidden="true"
					class="row-start-1 flex size-[30px] items-center justify-center rounded-pill bg-accent font-display text-body-sm font-semibold text-accent-foreground md:mt-2"
				>
					{index + 1}
				</span>
				<div class="col-span-4 min-w-0 md:col-span-1 md:col-start-3 md:row-start-1">
					<StepField {step} {index} {entries} {groups} error={errors[`step:${step.id}`]} />
				</div>
				<button
					type="button"
					onclick={() => removeStep(index)}
					aria-label={m.editor_step_remove()}
					class="col-start-4 row-start-1 flex items-center justify-center text-text-muted transition hover:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:mt-4"
				>
					<X class="size-4" aria-hidden="true" />
				</button>
				{#if errors[`step:${step.id}`]}
					<p
						id="step-error-{step.id}"
						class="col-span-4 -mt-1 text-micro font-medium text-destructive motion-safe:animate-error-in md:col-span-1 md:col-start-3"
					>
						{errors[`step:${step.id}`]}
					</p>
				{/if}
			</li>
		{/each}
	</ul>

	<div class="flex flex-wrap items-center gap-3">
		<Button variant="accent" onclick={() => addStep()} class="w-full md:w-auto">
			<Plus class="size-4" aria-hidden="true" />
			{m.editor_step_add()}
		</Button>
		<p class="text-caption text-text-muted">{m.editor_reference_hint()}</p>
	</div>
</div>
