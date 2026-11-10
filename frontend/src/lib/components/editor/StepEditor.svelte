<script lang="ts">
	import { flip } from 'svelte/animate';
	import { prefersReducedMotion } from 'svelte/motion';
	import { SvelteSet } from 'svelte/reactivity';
	import { tick } from 'svelte';
	import Check from '@lucide/svelte/icons/check';
	import GripVertical from '@lucide/svelte/icons/grip-vertical';
	import X from '@lucide/svelte/icons/x';
	import { dragHandle, dragHandleZone, type DndEvent } from 'svelte-dnd-action';
	import { m } from '#lib/paraglide/messages.js';
	import { applyDrag, isShadow, liftDragged, pickUp, slotHeight } from '#lib/recipe/dnd.js';
	import { newStep, type FieldErrors, type FormGroup, type FormStep } from '#lib/recipe/form.js';
	import type { PhotoTile } from '#lib/recipe/step-photos.js';
	import {
		acceptAll,
		countPending,
		waitingChoices,
		pickerEntries
	} from '#lib/recipe/step-references.js';
	import { ACCEPT, ACTION, LIST, ROW } from './link-list';
	import AddButton from './AddButton.svelte';
	import FieldError from '../ui/FieldError.svelte';
	import StepField from './StepField.svelte';
	import { collapse, fresh, reveal } from './motion';

	let {
		steps = $bindable([]),
		groups,
		tiles,
		errors = {}
	}: {
		steps?: FormStep[];
		/** The recipe's ingredient groups; every link points into one of them. */
		groups: FormGroup[];
		/** The recipe's photos a step's words may link to, as the images section shows them. */
		tiles: PhotoTile[];
		errors?: FieldErrors;
	} = $props();

	const FLIP_DURATION = $derived(prefersReducedMotion.current ? 0 : 150);

	const entries = $derived(pickerEntries(groups));
	const pendingCount = $derived(countPending(steps, groups, tiles));
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
	 * fall to `<body>`. It goes to a control that does not edit, never into a
	 * step's text, which would raise a phone's keyboard: the x of the step that
	 * takes the place (the next, or the previous when the last went), "Add
	 * step" when only one is left, or the x of the blank step that replaces
	 * the last one removed. A step among others folds shut first; it is looked
	 * up again afterwards, since the list can have changed while it closed.
	 * Steps still folding (`leaving`) are neither a focus target nor counted,
	 * so a quick second x does not send focus into a step on its way out.
	 */
	async function removeStep(index: number) {
		const step = steps[index];
		void announce(
			m.editor_announce_removed({ label: m.editor_step_number({ number: index + 1 }) })
		);
		const staying = steps.filter((s) => s.id === step.id || !leaving.has(s.id));
		if (staying.length > 1) {
			const at = staying.indexOf(step);
			const neighbor = staying[at + 1] ?? staying[at - 1];
			document
				.getElementById(staying.length === 2 ? 'step-add' : `step-remove-${neighbor.id}`)
				?.focus();
			leaving.add(step.id);
			await collapse(document.getElementById(`step-${step.id}`)?.closest('li'));
			leaving.delete(step.id);
		}
		const current = steps.findIndex((s) => s.id === step.id);
		if (current === -1) return;
		steps.splice(current, 1);
		if (steps.length === 0) {
			steps.push(newStep());
			await tick();
			document.getElementById(`step-remove-${steps[0].id}`)?.focus();
		}
	}

	/** The drop slot's height: see `slotHeight` in dnd.ts. */
	let slot = $state(0);

	function reorder(event: CustomEvent<DndEvent<FormStep>>) {
		slot = slotHeight(event) ?? slot;
		applyDrag(event, steps, (items) => (steps = items));
	}

	/** Steps folding shut after their x, still in `steps` until closed. */
	const leaving = new SvelteSet<string>();

	/** What "Accept all" or an x did, for a screen reader. */
	let said = $state('');

	/** Empties the region first: the same text again would not be read out. */
	async function announce(text: string) {
		said = '';
		await tick();
		said = text;
	}

	/**
	 * Takes every open proposal and says how many. The button goes with them,
	 * so focus moves to the box it was in while that stays for a waiting
	 * word, else to the first step's list toggle - not into a step, which
	 * would raise a phone's keyboard and open the card of a word at its start.
	 */
	function acceptEverything() {
		const count = acceptAll(steps, groups, tiles);
		void announce(m.editor_reference_accepted_all({ count }));
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
			circle that takes one proposal there takes them all here. Only its circle is
			the button, as on every row there; a save takes no proposal, so this
			is the one way to take them all at once.
		-->
		<div>
			<div id="accept-all-box" tabindex="-1" class="{LIST} outline-none">
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
	     them its announcements say "item" and "list". A
	     step a pressed handle picks up (`pickUp`) stands on a card 8px out
	     from its field, or from its links line when it shows one (not from
	     the empty line it reserves), its pill and × on the card's top edge
	     as they sit on the field's; the dragged copy is cloned with the card.
	     No morphing, which would size the copy to the slot. The pill and ×
	     are `data-spotlight-with`: a link sheet that lights up the field
	     lights them up whole (BottomSheet's `spotlight`). -->
	<ul
		use:dragHandleZone={{
			items: steps,
			type: 'step',
			flipDurationMs: FLIP_DURATION,
			dropTargetStyle: {},
			// The handles are the keyboard way in; the list itself is no Tab stop.
			zoneTabIndex: -1,
			morphDisabled: true,
			transformDraggedElement: liftDragged
		}}
		onconsider={reorder}
		onfinalize={reorder}
		aria-label={m.editor_section_steps()}
	>
		{#each steps as step, index (step.id)}
			<!--
				The number sits in a pill on the field's top border and is the drag
				handle; the remove button sits on the same border at the right, a
				ringed circle like the header's buttons. Both are centered on the
				line and stand 14px in, past the field's 12px corners, so the border
				runs from each corner into them. No row of their own, so a step costs
				only its field. Every step reserves the 36px of a links line under
				its field (`pb-9` = StepLinks' `mt-1` + `min-h-8`) and hands it back
				when it shows one (`data-links-line`), and an open links list drops
				its bottom margin, so steps stand equally apart with or without
				links. The reserve is padding, which `collapse`/`reveal` fold with
				the step. The top margin keeps a pill clear of the step above. No
				focus outline: a pointer drag focuses the copy it drags, which wears
				the card's outline instead; the handle carries the focus ring.
			-->
			<li
				data-spotlight-scope
				data-error-key="step:{step.id} steps"
				animate:flip={{ duration: fresh.size > 0 ? 0 : FLIP_DURATION }}
				in:reveal={{ fresh: fresh.has(step.id) }}
				aria-label={m.editor_step_number({ number: index + 1 })}
				class="relative mt-8 list-none pb-9 outline-none before:pointer-events-none before:absolute before:inset-0 before:bottom-9 before:-z-10 before:rounded-xl before:bg-surface before:opacity-0 before:outline-2 before:outline-primary before:transition-[inset,opacity,box-shadow] before:duration-160 before:ease-out has-[[data-links-line]]:pb-0 has-[[data-links-line]]:before:bottom-0 data-lift:isolate data-lift:z-10 data-[lift=up]:before:-inset-2 data-[lift=up]:before:bottom-7 data-[lift=up]:before:opacity-100 data-[lift=up]:before:shadow-lift has-[[data-links-line]]:data-[lift=up]:before:-bottom-2 motion-reduce:before:transition-none [&_ul[id^=step-links-]]:mb-0"
				style:height={isShadow(step) ? `${slot}px` : undefined}
			>
				{#if isShadow(step)}
					<!-- Where the dragged step will land: a slot of its height, empty.
					     The library hides the shadow element; the slot overrides that
					     to show itself. The look is the ingredient list's slot. The
					     slot is a child, not the item, because the library copies the
					     item's border and background onto the dragged copy. -->
					<div
						aria-hidden="true"
						class="visible h-full rounded-lg border-2 border-dashed border-handle bg-accent/40"
					></div>
				{:else}
					<!--
						A div with the button role, not a <button>: svelte-dnd-action
						ignores Space and Enter on anything with a `disabled` property, so
						a <button> handle could never start a keyboard drag. The first
						span widens the hit area to 44px (from the padding box, inside the 2px border).
					-->
					<div
						use:dragHandle
						data-spotlight-with
						onpointerdown={pickUp}
						role="button"
						tabindex="0"
						aria-label={m.editor_step_reorder()}
						class="absolute -top-3.5 left-3.5 z-10 h-7.5 cursor-grab touch-none rounded-pill border-2 border-surface bg-accent font-display text-body-sm font-semibold text-accent-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
					>
						<span aria-hidden="true" class="absolute -inset-x-1 -inset-y-2.25"></span>
						<span class="relative flex items-center gap-1 pr-2.5 pl-2">
							<GripVertical class="size-3.5 opacity-75" aria-hidden="true" />
							{index + 1}
						</span>
					</div>
					<button
						type="button"
						id="step-remove-{step.id}"
						data-spotlight-with
						onclick={() => removeStep(index)}
						aria-label={m.editor_step_remove()}
						class="absolute -top-3.5 right-3.5 z-10 flex size-7.5 items-center justify-center rounded-pill border border-border bg-surface-elevated text-text-muted transition after:absolute after:-inset-2 hover:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
					>
						<X class="size-4" aria-hidden="true" />
					</button>
					<StepField
						bind:step={steps[index]}
						{index}
						{entries}
						{groups}
						{tiles}
						error={errors[`step:${step.id}`]}
					/>
					{#if errors[`step:${step.id}`]}
						<FieldError
							id="step-error-{step.id}"
							message={errors[`step:${step.id}`]}
							class="mt-1"
						/>
					{/if}
				{/if}
			</li>
		{/each}
	</ul>

	<!-- The next step, drawn where it will land: as wide as a step's field,
	     its dot where the step's text starts. The ingredient list's add
	     button (IngredientGroupEditor). -->
	<AddButton id="step-add" onclick={() => addStep()} class="mt-4.5 h-11 md:h-10">
		{m.editor_step_add()}
	</AddButton>
	<p class="text-caption text-text-muted">{m.editor_reference_hint()}</p>
</div>
