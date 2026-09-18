<script lang="ts">
	import { prefersReducedMotion } from 'svelte/motion';
	import { scale } from 'svelte/transition';
	import Minus from 'lucide-svelte/icons/minus';
	import Plus from 'lucide-svelte/icons/plus';
	import { servingsUnit } from '$lib/recipe/format';
	import { SERVINGS_MAX, SERVINGS_MIN } from '$lib/recipe/servings';
	import { m } from '$lib/paraglide/messages';

	let {
		value,
		onchange,
		class: className = ''
	}: {
		/** Current servings, 1-99. */
		value: number;
		/** Called with the next value; the owner clamps and persists it. */
		onchange: (next: number) => void;
		class?: string;
	} = $props();

	const duration = $derived(prefersReducedMotion.current ? 0 : 150);
	const atMin = $derived(value <= SERVINGS_MIN);
	const atMax = $derived(value >= SERVINGS_MAX);

	/**
	 * The boundary buttons stay enabled and only announce `aria-disabled`:
	 * a truly `disabled` button loses focus the moment it is pressed at 1 or
	 * 99, which drops a keyboard user out of the stepper. The click is
	 * swallowed here instead.
	 */
	function step(delta: number) {
		const next = value + delta;
		if (next < SERVINGS_MIN || next > SERVINGS_MAX) {
			return;
		}
		onchange(next);
	}

	const buttonClass =
		'inline-flex size-8 shrink-0 items-center justify-center rounded-full transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] aria-disabled:opacity-40 aria-disabled:pointer-events-none';
</script>

<div
	role="group"
	aria-label={m.recipe_servings()}
	class="inline-flex h-10 items-center gap-1 rounded-pill bg-background p-1 {className}"
>
	<button
		type="button"
		aria-label={m.servings_decrease()}
		aria-disabled={atMin}
		onclick={() => step(-1)}
		class="{buttonClass} bg-surface text-text shadow-card"
	>
		<Minus class="size-4" aria-hidden="true" />
	</button>
	<!-- `{#key}` re-mounts the number on every change so the `in:` transition
	     runs; with no `out:` the old value leaves instantly and nothing
	     overlaps. `aria-live` reads the new count to screen readers. -->
	<span class="min-w-[104px] text-center" aria-live="polite">
		{#key value}
			<span class="inline-block" in:scale={{ duration, start: 0.8 }}>
				<span class="font-display text-body-lg font-semibold tabular-nums">{value}</span>
				<span class="text-caption text-text-muted">{servingsUnit(value)}</span>
			</span>
		{/key}
	</span>
	<button
		type="button"
		aria-label={m.servings_increase()}
		aria-disabled={atMax}
		onclick={() => step(1)}
		class="{buttonClass} bg-primary text-primary-foreground"
	>
		<Plus class="size-4" aria-hidden="true" />
	</button>
</div>
