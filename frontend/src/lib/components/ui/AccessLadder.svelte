<script lang="ts" generics="T extends string">
	let {
		value = $bindable(),
		options,
		label,
		description
	}: {
		value: T;
		/** Lowest level first; the first one is "nothing" and is never filled. */
		options: { value: T; label: string }[];
		/** Accessible name of the group. */
		label: string;
		/** One sentence saying what the chosen level allows. */
		description: string;
	} = $props();

	const descriptionId = $props.id();
	let buttons: HTMLButtonElement[] = $state([]);

	const depth = $derived(options.findIndex((option) => option.value === value));

	// Roving tabindex, as the WAI-ARIA radio group pattern expects: only the
	// checked step is in the tab order, arrows move and check. The same
	// handling as SegmentedControl.
	function handleKey(event: KeyboardEvent, index: number) {
		const delta =
			event.key === 'ArrowRight' || event.key === 'ArrowDown'
				? 1
				: event.key === 'ArrowLeft' || event.key === 'ArrowUp'
					? -1
					: 0;
		if (delta === 0) {
			return;
		}
		event.preventDefault();
		const next = (index + delta + options.length) % options.length;
		value = options[next].value;
		buttons[next]?.focus();
	}
</script>

<!--
	A picker for levels where each one includes every level below it, drawn as
	a gauge: the bars fill up to the chosen step, so "Write" visibly carries
	"Read". The labels sit under the bars instead of inside a pill, which is
	what lets four of them fit a 320px phone without being cut off; the
	sentence below says in words what the gauge shows.
-->
<div>
	<div role="radiogroup" aria-label={label} aria-describedby={descriptionId} class="flex gap-1">
		{#each options as option, i (option.value)}
			{@const checked = i === depth}
			<button
				bind:this={buttons[i]}
				type="button"
				role="radio"
				aria-checked={checked}
				tabindex={checked ? 0 : -1}
				onclick={() => (value = option.value)}
				onkeydown={(event) => handleKey(event, i)}
				class="group flex min-w-0 flex-1 flex-col items-stretch gap-1.5 rounded-sm pt-1.5 pb-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				<span
					aria-hidden="true"
					class="h-2.5 transition-colors group-first:rounded-l-pill group-last:rounded-r-pill {i ===
					0
						? `border-2 border-dashed ${checked ? 'border-primary' : 'border-border'}`
						: i <= depth
							? 'bg-primary'
							: 'bg-border group-hover:bg-handle'}"
				></span>
				<span
					class="text-caption font-semibold break-words hyphens-auto {checked
						? 'text-text'
						: 'text-text-muted group-hover:text-text'}"
				>
					{option.label}
				</span>
			</button>
		{/each}
	</div>
	<p id={descriptionId} class="mt-1 text-caption text-text-muted">{description}</p>
</div>
