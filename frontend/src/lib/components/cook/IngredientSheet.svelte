<script lang="ts">
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade, fly } from 'svelte/transition';
	import type { Recipe } from '$lib/api/recipes';
	import IngredientList from '$lib/components/recipe/IngredientList.svelte';
	import ServingsStepper from '$lib/components/recipe/ServingsStepper.svelte';
	import type { Servings } from '$lib/recipe/servings.svelte';
	import { m } from '$lib/paraglide/messages';

	let {
		open = $bindable(false),
		recipe,
		servings,
		wakeLockActive
	}: {
		open?: boolean;
		recipe: Recipe;
		/** The page's store - the same instance the step view will read in later phases. */
		servings: Servings;
		/** Shows the "Bildschirm bleibt an" caption only while a lock is really held. */
		wakeLockActive: boolean;
	} = $props();

	const count = $derived(
		recipe.ingredientGroups.reduce((n, group) => n + group.ingredients.length, 0)
	);
	const duration = $derived(prefersReducedMotion.current ? 0 : 250);
	/** The backdrop fades a touch faster than the panel flies in. */
	const fadeDuration = $derived(prefersReducedMotion.current ? 0 : 200);

	// Move focus into the panel when it opens so keyboard users land on the
	// sheet, and back onto the collapsed card when it closes, so they never
	// have to find their place again.
	let restoreFocus = $state(false);

	function focusOnMount(node: HTMLElement) {
		node.focus();
	}

	/** Every close path goes through here, so focus always comes back. */
	function close() {
		restoreFocus = true;
		open = false;
	}

	// The sheet owns Escape while it is open: it closes the list instead of
	// leaving cooking mode. The page's own handler stands down for as long
	// as `open` is true.
	function onkeydown(event: KeyboardEvent) {
		if (!open || event.key !== 'Escape') {
			return;
		}
		event.preventDefault();
		close();
	}

	const handleClass = 'flex w-full flex-col items-center px-5 pt-2 pb-3';
</script>

<svelte:window {onkeydown} />

<!-- The collapsed card stays mounted while the sheet is open, only invisible:
     unmounting it would take its height out of the column and shift the step
     underneath. `visibility: hidden` also removes it from the tab order and
     the accessibility tree, so the open sheet's `aria-modal` stays truthful. -->
<button
	type="button"
	onclick={() => (open = true)}
	aria-expanded={open}
	class="mx-auto w-full max-w-[640px] shrink-0 rounded-t-3xl bg-background shadow-sheet {handleClass} {open
		? 'invisible'
		: ''}"
	{@attach (node) => {
		if (restoreFocus) {
			node.focus();
			restoreFocus = false;
		}
	}}
>
	<span class="h-1 w-9 rounded-pill bg-handle" aria-hidden="true"></span>
	<span class="mt-2 text-body-sm font-semibold">{m.cook_sheet_summary({ count })}</span>
	{#if wakeLockActive}
		<span class="mt-0.5 text-label tracking-normal text-text-muted">{m.cook_wake_lock()}</span>
	{/if}
</button>

{#if open}
	<button
		type="button"
		aria-label={m.cook_sheet_close()}
		onclick={close}
		class="fixed inset-0 z-40 bg-overlay"
		transition:fade={{ duration: fadeDuration }}
	></button>
	<div
		role="dialog"
		aria-modal="true"
		aria-label={m.recipe_ingredients()}
		tabindex="-1"
		{@attach focusOnMount}
		class="fixed inset-x-0 bottom-0 z-50 mx-auto flex max-h-[80dvh] w-full max-w-[640px] flex-col rounded-t-3xl bg-background shadow-sheet outline-none"
		transition:fly={{ y: 200, duration }}
	>
		<button type="button" onclick={close} aria-expanded="true" class={handleClass}>
			<span class="h-1 w-9 rounded-pill bg-handle" aria-hidden="true"></span>
			<span class="mt-2 text-body-sm font-semibold">{m.cook_sheet_summary({ count })}</span>
		</button>
		<div class="flex items-center gap-3 px-5 pb-3">
			<ServingsStepper value={servings.value} onchange={(next) => servings.set(next)} />
			{#if servings.scaled}
				<button
					type="button"
					onclick={() => servings.reset()}
					class="text-body-sm font-medium text-primary underline-offset-4 transition hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					{m.servings_reset()}
				</button>
			{/if}
		</div>
		<div class="min-h-0 flex-1 overflow-y-auto px-4 pb-6">
			<IngredientList
				recipeId={recipe.id}
				groups={recipe.ingredientGroups}
				servings={servings.value}
				baseServings={servings.base}
			/>
		</div>
	</div>
{/if}
