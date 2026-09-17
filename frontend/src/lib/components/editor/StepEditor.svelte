<script lang="ts">
	import { flip } from 'svelte/animate';
	import { tick } from 'svelte';
	import GripVertical from 'lucide-svelte/icons/grip-vertical';
	import Plus from 'lucide-svelte/icons/plus';
	import X from 'lucide-svelte/icons/x';
	import { dragHandle, dragHandleZone, type DndEvent } from 'svelte-dnd-action';
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';
	import { newStep, type FormStep } from '$lib/recipe/form';

	let {
		steps = $bindable([])
	}: {
		steps?: FormStep[];
	} = $props();

	const FLIP_DURATION = 150;

	/**
	 * Grows the textarea to fit its content. Taking the text as an argument
	 * makes the attachment re-run whenever the step changes - typing, a paste,
	 * or a reorder that moves different text into this element.
	 */
	function autogrow(text: string) {
		return (node: HTMLTextAreaElement) => {
			void text;
			node.style.height = 'auto';
			node.style.height = `${node.scrollHeight}px`;
		};
	}

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
		steps.splice(index, 1);
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
</script>

<div class="space-y-3">
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
			<li
				animate:flip={{ duration: FLIP_DURATION }}
				aria-label={m.editor_step_number({ number: index + 1 })}
				class="grid list-none grid-cols-[20px_32px_1fr_28px] items-start gap-2"
			>
				<button
					use:dragHandle
					type="button"
					aria-label={m.editor_step_reorder()}
					class="mt-4 flex cursor-grab items-center justify-center text-handle focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<GripVertical class="size-4" aria-hidden="true" />
				</button>
				<span
					aria-hidden="true"
					class="mt-2 flex size-[30px] items-center justify-center rounded-pill bg-accent font-display text-body-sm font-semibold text-accent-foreground"
				>
					{index + 1}
				</span>
				<textarea
					id="step-{step.id}"
					bind:value={step.text}
					{@attach autogrow(step.text)}
					rows="2"
					aria-label={m.editor_step_number({ number: index + 1 })}
					placeholder={m.editor_step_placeholder()}
					class="min-h-16 w-full resize-none overflow-hidden rounded-md border border-border bg-surface-elevated px-3.5 py-2.5 text-body transition outline-none focus:border-primary"
				></textarea>
				<button
					type="button"
					onclick={() => removeStep(index)}
					aria-label={m.editor_step_remove()}
					class="mt-4 flex items-center justify-center text-text-muted transition hover:text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<X class="size-4" aria-hidden="true" />
				</button>
			</li>
		{/each}
	</ul>

	<Button variant="accent" onclick={() => addStep()} class="w-full md:w-auto">
		<Plus class="size-4" aria-hidden="true" />
		{m.editor_step_add()}
	</Button>
</div>
