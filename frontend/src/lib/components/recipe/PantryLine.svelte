<script module lang="ts">
	// One canvas and one font listener serve every card's lines. Fonts that
	// land late set the names wider, and each face is its own file (the bold
	// one too), so every load starts the measure over, not only the first
	// `ready`. Registered with the first line and kept for the page's life.
	let canvas: CanvasRenderingContext2D | null = null;
	let fonts = $state(0);
	let listening = false;
	function listen() {
		if (listening) return;
		listening = true;
		canvas = document.createElement('canvas').getContext('2d');
		const bump = () => fonts++;
		void document.fonts.ready.then(bump);
		document.fonts.addEventListener('loadingdone', bump);
	}
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLAttributes } from 'svelte/elements';
	import FindParts from './FindParts.svelte';
	import type { FindPart } from '#lib/api/recipes.js';
	import { m } from '#lib/paraglide/messages.js';
	import { clampNames, itemText, longNames } from '#lib/recipe/finds.js';

	let {
		items,
		lines,
		lead,
		empty,
		...rest
	}: {
		/** The names, each in parts with its hits marked. */
		items: { parts: FindPart[] }[];
		/** How many lines the names may take before they end in "+N". */
		lines: 2 | 3;
		/** The phone's icon in front of the names; hidden from md up. */
		lead: Snippet;
		/** What stands in the line when it names nothing. */
		empty?: Snippet;
	} & HTMLAttributes<HTMLSpanElement> = $props();

	const names = $derived(items.map(itemText));

	// The line holds `lines` lines at most and drops whole names from its
	// end until it fits, ending in "+N". Measured, since only the layout
	// knows where a line breaks: each pass drops one name and the effect
	// looks again. A new width, new fonts or new parts start over from all
	// of them: a part turned bold (the tick row's hovered pill) sets its
	// name wider and can push the line past its last one.
	let box = $state<HTMLElement>();
	let width = $state(0);
	$effect(listen);

	// The box's font and the room its lead icon takes, read from the layout
	// once per width and font load, in an effect: read while rendering, a
	// tick, which changes every card, forced a layout per card.
	let metrics = $state.raw({ font: '', weight: '', lead: 0 });
	$effect(() => {
		void width;
		void fonts;
		if (!box) return;
		const style = getComputedStyle(box);
		// The icon leads the box on the phone; on desk it is hidden.
		const icon = box.firstElementChild;
		const shown = icon?.getBoundingClientRect().width ?? 0;
		metrics = {
			font: `${style.fontSize} ${style.fontFamily}`,
			weight: style.fontWeight,
			lead: icon && shown > 0 ? shown + parseFloat(getComputedStyle(icon).marginRight) : 0
		};
	});

	const key = $derived(
		`${width}\n${fonts}\n${metrics.font}\n${metrics.lead}\n${JSON.stringify(items)}`
	);
	let fit = $state({ key: '', fits: Infinity });
	const clamped = $derived(clampNames(names, fit.key === key ? fit.fits : Infinity));
	$effect(() => {
		const shown = clamped.shown.length;
		if (box && shown > 1 && box.scrollHeight > box.clientHeight + 1) {
			fit = { key, fits: shown - 1 };
		}
	});

	// Only a name too wide for the room it starts in may hyphenate: the room
	// after the phone's icon for the first, a full line for the rest.
	// Measured on the canvas in the box's font, bold for a name with a hit.
	const long = $derived.by(() => {
		const { font, weight, lead } = metrics;
		if (!font || !canvas || width === 0) return [];
		const ctx = canvas;
		return longNames(names, { first: width - lead, line: width }, (text, i) => {
			const bold = items[i].parts.some((part) => part.hit);
			ctx.font = `${bold ? 700 : weight} ${font}`;
			return ctx.measureText(text).width;
		});
	});
</script>

<!-- The lead icon stands first, where the long-name measure finds it.
     The separators are mustaches: Svelte trims a literal space at the edge
     of a block. -->
<!-- eslint-disable svelte/no-useless-mustaches -->
<span
	bind:this={box}
	bind:clientWidth={width}
	{...rest}
	class="block overflow-hidden text-caption text-text [hyphenate-limit-chars:auto_6_7] {lines === 2
		? 'max-h-[2lh]'
		: 'max-h-[3lh]'}"
	>{@render lead()}{#if items.length === 0}{@render empty?.()}{:else}<FindParts
			items={items.slice(0, clamped.shown.length).map((item, i) => ({ ...item, long: long[i] }))}
		/>{#if clamped.more > 0}{' '}<span class="font-semibold whitespace-nowrap text-text-muted"
				>{m.pantry_more({ count: clamped.more })}</span
			>{/if}{/if}</span
>
<!-- eslint-enable svelte/no-useless-mustaches -->
