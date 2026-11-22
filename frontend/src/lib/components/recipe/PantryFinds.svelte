<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import List from '@lucide/svelte/icons/list';
	import FindParts from './FindParts.svelte';
	import type { PantryItem, SearchFind } from '#lib/api/recipes.js';
	import { m } from '#lib/paraglide/messages.js';
	import { clampNames, itemText, missingParts } from '#lib/recipe/finds.js';

	let {
		used,
		missing,
		finds = [],
		photo = false,
		highlightKey = null
	}: {
		/** The recipe's own names for what the chips cover. */
		used: string[];
		/** What the recipe still needs, after the ticks. */
		missing: PantryItem[];
		/** The card's text finds: a word found in an ingredient named here is marked in its line. */
		finds?: SearchFind[];
		photo?: boolean;
		/** The ingredient the tick row's hovered pill names, marked in the missing line. */
		highlightKey?: string | null;
	} = $props();

	const hits = $derived(
		new Map(
			finds
				.filter((find) => find.field === 'ingredients')
				.flatMap((find) => find.items)
				.map((item) => [itemText(item), item.parts])
		)
	);
	const names = $derived(missing.map((item) => item.name));

	// The missing line holds three lines at most and drops whole names from
	// its end until it fits, ending in "+N". Measured, since only the layout
	// knows where a line breaks: each pass drops one name and the effect
	// looks again. A new width or new names start over from all of them.
	let box = $state<HTMLElement>();
	let width = $state(0);
	// Fonts that land late set the names wider; the measure starts over then.
	let fontsReady = $state(false);
	$effect(() => {
		void document.fonts.ready.then(() => (fontsReady = true));
	});
	const key = $derived(`${width}\n${fontsReady}\n${names.join('\n')}`);
	let fit = $state({ key: '', fits: Infinity });
	const clamped = $derived(clampNames(names, fit.key === key ? fit.fits : Infinity));
	$effect(() => {
		const shown = clamped.shown.length;
		if (box && shown > 1 && box.scrollHeight > box.clientHeight + 1) {
			fit = { key, fits: shown - 1 };
		}
	});

	const label = $derived(
		`sr-only items-center gap-1 text-micro font-medium desk:not-sr-only desk:flex ${photo ? 'text-text-muted' : 'text-text/75'}`
	);
</script>

<!-- What the recipe has of the reader's ingredients and what it still
     needs, in `SearchFinds`' look: a quiet label row over each line. A
     two-column phone card has no room for label rows, so there the check
     mark leads the first line and "Missing:" the second, and the stamp
     alone carries the numbers. The reader's own ingredients are marked
     like a search hit. -->
<ul class="mt-1.5 flex flex-col desk:mt-3 desk:gap-2">
	{#if used.length > 0}
		<li
			class="flex min-w-0 items-center gap-1 desk:flex-col desk:items-stretch desk:gap-0.5 desk:border-l-2 desk:border-primary/45 desk:py-px desk:pl-2.5"
		>
			<span class={label}>
				<Check class="size-3.5 shrink-0 text-success-foreground" aria-hidden="true" />
				<span class="truncate">{m.pantry_yours()}<span class="sr-only">:</span></span>
			</span>
			<Check
				class="size-3.5 shrink-0 stroke-[2.6] text-success-foreground desk:hidden"
				aria-hidden="true"
			/>
			<span class="truncate text-caption text-text" title={used.join(', ')}>
				<FindParts items={used.map((name) => ({ parts: [{ text: name, hit: true }] }))} />
			</span>
		</li>
	{/if}
	<li
		class="flex min-w-0 flex-col desk:gap-0.5 desk:border-l-2 desk:border-handle desk:py-px desk:pl-2.5"
	>
		<span class={label}>
			<List class="size-3.5 shrink-0" aria-hidden="true" />
			<span class="truncate">{m.pantry_missing_label()}<span class="sr-only">:</span></span>
		</span>
		<span
			bind:this={box}
			bind:clientWidth={width}
			data-missing
			class="block max-h-[3lh] overflow-hidden text-caption text-text"
		>
			<!-- The separators are mustaches: Svelte trims a literal space at the
			     edge of a block. -->
			<!-- eslint-disable svelte/no-useless-mustaches -->
			<span aria-hidden="true" class="font-semibold desk:hidden">{m.pantry_missing()}{' '}</span
			>{#if names.length === 0}<span class="text-text-muted">{m.pantry_nothing_missing()}</span
				>{:else}<FindParts
					items={missingParts(missing.slice(0, clamped.shown.length), hits, highlightKey)}
				/>{#if clamped.more > 0}{' '}<span class="font-semibold whitespace-nowrap text-text-muted"
						>{m.pantry_more({ count: clamped.more })}</span
					>{/if}{/if}
			<!-- eslint-enable svelte/no-useless-mustaches -->
		</span>
	</li>
</ul>
