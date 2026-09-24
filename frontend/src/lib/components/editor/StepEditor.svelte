<script lang="ts">
	import { flip } from 'svelte/animate';
	import { tick } from 'svelte';
	import GripVertical from 'lucide-svelte/icons/grip-vertical';
	import Plus from 'lucide-svelte/icons/plus';
	import X from 'lucide-svelte/icons/x';
	import { dragHandle, dragHandleZone, type DndEvent } from 'svelte-dnd-action';
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';
	import { newStep, type FieldErrors, type FormGroup, type FormStep } from '$lib/recipe/form';
	import {
		acceptAll,
		countPending,
		pickerEntries,
		type DismissedWords
	} from '$lib/recipe/step-references';
	import StepField from './StepField.svelte';

	let {
		steps = $bindable([]),
		groups,
		dismissed = $bindable({}),
		errors = {}
	}: {
		steps?: FormStep[];
		/** The recipe's ingredient groups; every link points into one of them. */
		groups: FormGroup[];
		/**
		 * Proposals the author turned down, keyed by step id. Bound, because
		 * the save button in `RecipeForm` counts what is still open and has to
		 * see the same set.
		 */
		dismissed?: DismissedWords;
		errors?: FieldErrors;
	} = $props();

	const FLIP_DURATION = 150;

	const entries = $derived(pickerEntries(groups));
	const pendingCount = $derived(countPending(steps, groups, dismissed));

	async function addStep(index = steps.length) {
		const step = newStep();
		steps.splice(index, 0, step);
		await tick();
		document.getElementById(`step-${step.id}`)?.focus();
	}

	/**
	 * Removing a step destroys the x button that had focus, so focus would
	 * fall to `<body>`. Hand it to the step that took its place: the one
	 * above, or - when the first step went - the one that moved up into index
	 * 0, which is also the fresh blank step when the list was emptied.
	 */
	async function removeStep(index: number) {
		const [removed] = steps.splice(index, 1);
		// The dismissals went with that step, and ids are never reused, so
		// keeping them would only grow a map nothing reads again.
		if (removed && removed.id in dismissed) {
			dismissed = Object.fromEntries(
				Object.entries(dismissed).filter(([stepId]) => stepId !== removed.id)
			);
		}
		if (steps.length === 0) {
			steps.push(newStep());
		}
		const neighbour = steps[Math.max(0, index - 1)];
		await tick();
		document.getElementById(`step-${neighbour.id}`)?.focus();
	}

	function reorder(event: CustomEvent<DndEvent<FormStep>>) {
		steps = event.detail.items;
	}

	/** Turns one proposal down for one step; it stays down for this session. */
	function dismiss(stepId: string, word: string) {
		const words = dismissed[stepId] ?? [];
		if (words.includes(word)) return;
		dismissed = { ...dismissed, [stepId]: [...words, word] };
	}
</script>

<div class="space-y-3">
	{#if pendingCount > 0}
		<!--
			The one place the matcher's work is visible as a whole, and the only
			place that says what saving will do with it: the sentence belongs
			beside the proposals it is about, where the author already is while
			reviewing them, and it costs nothing on a screen that has none.

			The sentence and the button deliberately say different things. The
			proposals are taken on save either way; the button only takes them
			now, so the links can be looked over - and single ones removed -
			before the recipe is written.

			Stacked on a phone and a row from `sm:` up: the sentence and the
			button together are wider than a narrow screen, and a wrapped row
			would leave a word of the sentence stranded beside the button.

			Stacked, the sentence carries `pl-5` so its first letter lands under
			the button's: the button's box starts at the same content edge, but
			its own `px-5` insets the label 20px further, and 20px is exactly
			what this adds on top of the container's `px-3`. It is dropped again
			from `sm:` up, where the two sit on one row and the question does
			not arise. Padding the text is the fix rather than pulling the
			button left, which would take it past the container's own edge -
			and rather than taking padding off `Button`, which the whole app
			shares.
		-->
		<div
			class="flex flex-col items-start gap-2 rounded-md border border-dashed border-primary px-3 py-2 sm:flex-row sm:items-center sm:justify-between"
		>
			<p class="pl-5 text-body-sm text-primary sm:pl-0">
				{pendingCount === 1
					? m.editor_reference_save_takes_one()
					: m.editor_reference_save_takes({ count: pendingCount })}
			</p>
			<Button variant="ghost" onclick={() => acceptAll(steps, groups, dismissed)}>
				{m.editor_reference_accept_now()}
			</Button>
		</div>
	{/if}

	<!-- The `aria-label`s on the zone and on every item are what
	     svelte-dnd-action reads out while dragging (see its README); without
	     them its announcements say "item" and "list". -->
	<ul
		use:dragHandleZone={{ items: steps, flipDurationMs: FLIP_DURATION, dropTargetStyle: {} }}
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
				animate:flip={{ duration: FLIP_DURATION }}
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
					<StepField
						{step}
						{index}
						{entries}
						{groups}
						dismissed={dismissed[step.id] ?? []}
						error={errors[`step:${step.id}`]}
						ondismiss={(word) => dismiss(step.id, word)}
					/>
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
						class="col-span-4 -mt-1 text-micro font-medium text-destructive md:col-span-1 md:col-start-3"
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
