<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import List from '@lucide/svelte/icons/list';
	import { m } from '#lib/paraglide/messages.js';

	let {
		title,
		missing,
		pressed = false,
		onpress
	}: {
		/** The recipe's, so the button names which card it focuses. */
		title: string;
		/** How many ingredients the recipe still needs. */
		missing: number;
		pressed?: boolean;
		/** Without it the stamp is a label, not a button. */
		onpress?: () => void;
	} = $props();

	const all = $derived(missing === 0);
	const state = $derived(
		all
			? m.pantry_all_there()
			: missing === 1
				? m.pantry_missing_count_one()
				: m.pantry_missing_count({ count: missing })
	);
	const look = $derived(
		`inline-flex h-6 shrink-0 items-center gap-1 rounded-pill text-micro font-bold whitespace-nowrap shadow-card desk:h-7 ${
			pressed
				? 'bg-primary text-primary-foreground'
				: all
					? 'bg-success-soft text-success-foreground'
					: 'bg-surface text-text'
		} ${all ? 'pr-2.5 pl-2' : 'pr-2.5 pl-1.5'}`
	);
</script>

{#snippet content()}
	{#if all}
		<Check class="size-3.5 stroke-3" aria-hidden="true" />{state}
	{:else}
		<List class="size-4" aria-hidden="true" />{state}
	{/if}
{/snippet}

<!-- The stamp says how much of the recipe the reader still lacks. As a
     button it focuses the tick row on this card, a toggle like the star:
     the name stays, `aria-pressed` carries the state. As tall as the
     card's star, so both sit concentric with their corners (see
     RecipeCard); its `::after` grows the pill to a 44px target.
     `preventDefault`/`stopPropagation` keep a tap from reaching the
     card's link underneath. -->
{#if onpress}
	<button
		type="button"
		aria-pressed={pressed}
		aria-label={m.pantry_stamp_label({ title, state })}
		data-stamp
		onclick={(event) => {
			event.preventDefault();
			event.stopPropagation();
			onpress();
		}}
		class="{look} relative hover-tint transition after:absolute after:-inset-x-1.5 after:-inset-y-2.5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
	>
		{@render content()}
	</button>
{:else}
	<span class={look}>{@render content()}</span>
{/if}
