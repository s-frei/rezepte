<script lang="ts">
	import { onMount } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade, fly } from 'svelte/transition';
	import { afterNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import ArrowRight from '@lucide/svelte/icons/arrow-right';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import CookProgress from '$lib/components/cook/CookProgress.svelte';
	import IngredientSheet from '$lib/components/cook/IngredientSheet.svelte';
	import TypeSpecimen from '$lib/components/cook/TypeSpecimen.svelte';
	import StepText from '$lib/components/recipe/StepText.svelte';
	import IconButton from '$lib/components/ui/IconButton.svelte';
	import { createServings } from '$lib/recipe/servings.svelte';
	import { swipeDirection } from '$lib/recipe/swipe';
	import { DEFAULT_TYPE_SIZE, TYPE_SIZE_CLASSES, type TypeSize } from '$lib/recipe/type-size';
	import { ScreenWakeLock } from '$lib/recipe/wake-lock.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const recipe = $derived(data.recipe);
	const detailHref = $derived(resolve('/recipes/[slug]', { slug: recipe.slug }));
	const count = $derived(recipe.steps.length);
	// Same storage key as the detail page, so "Scaling persists" (spec §7)
	// without passing anything through the URL.
	const servings = $derived(createServings(recipe.id, recipe.servings));

	let index = $state(0);
	let sheetOpen = $state(false);
	// Held for this visit only: every visit to cook mode starts at the
	// default, and a long step is sized down for the cook it is in.
	let typeSize = $state<TypeSize>(DEFAULT_TYPE_SIZE);
	let specimenOpen = $state(false);
	// Which way the next step slides in: +1 forward, -1 back.
	let direction = $state<1 | -1>(1);
	const isLast = $derived(index >= count - 1);
	const duration = $derived(prefersReducedMotion.current ? 0 : 200);

	function go(next: number) {
		if (next < 0 || next >= count || next === index) {
			return;
		}
		direction = next > index ? 1 : -1;
		index = next;
	}

	// Coming from the detail page pushed a history entry. Leaving with `goto`
	// would push a second one, so the browser's Back button would drop the
	// cook straight back into cooking mode; going back pops the entry we
	// added instead. Entered directly (bookmark, reload, deep link) there is
	// nothing to pop, so the detail page replaces cooking mode in place.
	let cameFromDetail = $state(false);

	afterNavigate(({ from }) => {
		cameFromDetail = from?.route.id === '/recipes/[slug]';
	});

	function finish() {
		if (cameFromDetail) {
			history.back();
		} else {
			void goto(detailHref, { replaceState: true });
		}
	}

	function next() {
		if (isLast) {
			finish();
		} else {
			go(index + 1);
		}
	}

	function onkeydown(event: KeyboardEvent) {
		// The sheet's own listener may already have handled the key (Escape).
		if (event.defaultPrevented) {
			return;
		}
		// Cmd+Left and Alt+Left are the browser's own Back; never page a step
		// (or leave cooking mode) out from under a browser shortcut.
		if (event.altKey || event.metaKey || event.ctrlKey) {
			return;
		}
		// The open sheet owns the keyboard: it closes itself on Escape instead
		// of leaving cook mode, and the arrows must not page steps the cook
		// cannot see. The open specimen owns the arrows for its radio group.
		if (sheetOpen || specimenOpen) {
			return;
		}
		switch (event.key) {
			case 'ArrowRight':
				event.preventDefault();
				go(index + 1);
				break;
			case 'ArrowLeft':
				event.preventDefault();
				go(index - 1);
				break;
			case 'Escape':
				event.preventDefault();
				finish();
				break;
		}
	}

	// Pointer swipe, same shape as the lightbox: remember where the pointer
	// went down, decide on up; a canceled or departed pointer forgets the
	// start so the next unrelated `pointerup` is not measured against it.
	let swipeStart: number | null = null;

	function onpointerdown(event: PointerEvent) {
		swipeStart = event.clientX;
	}

	function onpointerup(event: PointerEvent) {
		if (swipeStart === null) {
			return;
		}
		const swipe = swipeDirection(swipeStart, event.clientX);
		swipeStart = null;
		if (swipe === 'next') {
			go(index + 1);
		} else if (swipe === 'previous') {
			go(index - 1);
		}
	}

	function cancelSwipe() {
		swipeStart = null;
	}

	const wakeLock = new ScreenWakeLock();

	onMount(() => {
		void wakeLock.acquire();
		// The browser drops the lock when the tab is hidden; take it again
		// when the cook comes back.
		function onVisibilityChange() {
			if (document.visibilityState === 'visible') {
				void wakeLock.acquire();
			}
		}
		document.addEventListener('visibilitychange', onVisibilityChange);
		return () => {
			document.removeEventListener('visibilitychange', onVisibilityChange);
			void wakeLock.release();
		};
	});

	const navButtonClass =
		'inline-flex h-14 items-center justify-center gap-2 rounded-pill text-body font-semibold transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] disabled:pointer-events-none disabled:opacity-40';
</script>

<svelte:head>
	<title>{recipe.title} · {m.detail_cook_mode()}</title>
</svelte:head>

<svelte:window {onkeydown} />

<div class="flex h-dvh flex-col bg-surface text-text">
	<!-- `display: contents`, so the regions stay flex items of the column while
	     one `inert` makes the whole page behind the sheet unreachable - which
	     is what the sheet's `aria-modal="true"` promises. -->
	<div inert={sheetOpen} class="contents">
		<header class="flex shrink-0 items-center gap-3 px-5 pt-4 md:px-8">
			<!-- The header sits on `bg-surface`; the button needs the lighter
			     `bg-background` to be visible against it. -->
			<IconButton label={m.cook_back()} onclick={finish} class="bg-background">
				<ArrowLeft class="size-5" aria-hidden="true" />
			</IconButton>
			<p class="flex-1 text-center text-body-sm font-semibold text-text-muted" aria-live="polite">
				{#if count > 0}
					{m.cook_step_counter({ number: index + 1, count })}
				{/if}
			</p>
			<!-- Same width as the back button, so the counter stays centered. -->
			{#if count > 0}
				<TypeSpecimen bind:value={typeSize} bind:open={specimenOpen} />
			{:else}
				<span class="size-10 shrink-0" aria-hidden="true"></span>
			{/if}
		</header>

		{#if count > 0}
			<div class="shrink-0 px-5 pt-3 md:px-8">
				<CookProgress {count} current={index} />
			</div>

			<!-- `role="group"`: a div with pointer handlers needs a role
			     (a11y_no_static_element_interactions). The `grid` stacks the
			     outgoing and incoming step on one cell so the fade-out and the
			     slide-in overlap without a layout jump. -->
			<div
				role="group"
				class="grid min-h-0 flex-1 touch-pan-y overflow-hidden px-5 md:px-8"
				{onpointerdown}
				{onpointerup}
				onpointercancel={cancelSwipe}
				onpointerleave={cancelSwipe}
			>
				{#key index}
					<!-- `safe center`: a step taller than the view starts at the top
					     and scrolls, instead of centering its first lines out of reach. -->
					<section
						class="flex flex-col items-center justify-center-safe overflow-y-auto py-6 text-center [grid-area:1/1]"
						in:fly={{ x: 40 * direction, duration }}
						out:fade={{ duration }}
					>
						<h1 class="font-display text-body-lg text-primary italic">{recipe.title}</h1>
						<!-- Hyphenation only for words of twelve characters or more, split no
						     closer than four from either end: a long German compound
						     breaks at the largest size instead of being clipped, while
						     "potatoes" never turns into "pota-toes" on a line that had
						     room. Browsers without `hyphenate-limit-chars` hyphenate
						     every word instead. -->
						<p
							class="mt-4 max-w-[640px] font-display {TYPE_SIZE_CLASSES[
								typeSize
							]} leading-[1.3] font-medium wrap-break-word hyphens-auto [hyphenate-limit-chars:12_4_4]"
						>
							<StepText
								step={recipe.steps[index]}
								groups={recipe.ingredientGroups}
								servings={servings.value}
								baseServings={servings.base}
							/>
						</p>
					</section>
				{/key}
			</div>

			<footer class="flex shrink-0 items-center justify-center gap-4 px-5 pt-2 pb-3 md:px-8">
				<button
					type="button"
					onclick={() => go(index - 1)}
					disabled={index === 0}
					aria-label={m.cook_previous()}
					class="{navButtonClass} size-14 shrink-0 bg-background text-text"
				>
					<ChevronLeft class="size-6" aria-hidden="true" />
				</button>
				<button
					type="button"
					onclick={next}
					class="{navButtonClass} w-full max-w-[320px] px-8 {isLast
						? 'bg-primary text-primary-foreground'
						: 'bg-inverse text-inverse-foreground'}"
				>
					{isLast ? m.cook_finish() : m.cook_next()}
					{#if !isLast}
						<ArrowRight class="size-5" aria-hidden="true" />
					{/if}
				</button>
			</footer>
		{:else}
			<p class="m-auto px-5 text-center text-body text-text-muted">{m.cook_no_steps()}</p>
		{/if}
	</div>

	<IngredientSheet bind:open={sheetOpen} {recipe} {servings} wakeLockActive={wakeLock.active} />
</div>
