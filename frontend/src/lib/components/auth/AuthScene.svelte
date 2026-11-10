<script lang="ts">
	import { flushSync, tick, untrack, type Snippet } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MediaQuery } from 'svelte/reactivity';
	import kitchen from '$brand/lockups/rezepte-kitchen.webp?url';
	import mascot from '$brand/lockups/rezepte-mascot.webp?url';
	import wordmark from '$brand/lockups/rezepte-wordmark.svg?url';
	import { m } from '#lib/paraglide/messages.js';
	import { followVisibleArea, isTextField, type VisibleArea } from '#lib/visible-area.js';
	import { pullToClose } from '#lib/components/ui/pull-to-close.js';

	// The platform's close request (Android's Back); Chromium only, not yet in
	// TypeScript's DOM types.
	type CloseWatcher = { onclose: (() => void) | null; destroy(): void };
	type CloseWatchers = { CloseWatcher?: new () => CloseWatcher };

	let {
		lead = '',
		frame,
		open = $bindable(false),
		labelledby,
		cover,
		still = false,
		children
	}: {
		/** The line under the wordmark; left out when empty. */
		lead?: string;
		/**
		 * Below lg, the page as a frame the size of the visible area instead of
		 * a flowing page: `sheet` opens straight in the sheet under the plate,
		 * `cover` opens on the cover and raises the sheet while `open`. Left
		 * out, the page flows and scrolls as any other.
		 */
		frame?: 'sheet' | 'cover';
		/** With `frame="cover"`: the sheet is up. Closing it sets this back. */
		open?: boolean;
		/** With `frame="cover"`: the id of the card's heading, naming the sheet. */
		labelledby?: string;
		/** With `frame="cover"`, below lg: the cover's controls under the lead. */
		cover?: Snippet;
		/**
		 * Holds the frame as it is drawn: the keyboard leaving and focus moving
		 * no longer move the plate or the sheet (the login's greeting growing
		 * out of the sheet, with the frame still around it).
		 */
		still?: boolean;
		/** The card's contents. */
		children: Snippet;
	} = $props();

	// The same query as the style block's and Tailwind's `lg`, so no width
	// falls between the two layouts.
	const phone = new MediaQuery('width < 64rem');
	const framed = $derived(frame !== undefined && phone.current);
	// The cover's sheet, a dialog over the plate while it is up.
	const dialog = $derived(framed && frame === 'cover' && open);

	// The frame follows the visible area, so the software keyboard shrinks it
	// rather than covering it, and the document never scrolls under it.
	let main: HTMLElement;
	let sheet: HTMLElement;
	let probe = $state<HTMLElement>();
	let coverParts = $state<HTMLElement>();
	let area = $state<VisibleArea>({ top: 0, bottom: 0 });

	// The focused field stays in view inside the sheet once the frame has
	// taken its new size.
	async function reveal() {
		await tick();
		const focused = document.activeElement;
		if (focused instanceof HTMLElement && sheet.contains(focused)) {
			focused.scrollIntoView({ block: 'nearest' });
		}
	}

	$effect(() => {
		if (!framed || !probe) return;
		const root = document.documentElement;
		root.style.overflow = 'hidden';
		// It measures on focusin too, so this also covers a field taking focus.
		const stop = followVisibleArea(probe, (next) => {
			if (still) return;
			area = next;
			void reveal();
		});
		return () => {
			stop();
			root.style.overflow = '';
			area = { top: 0, bottom: 0 };
		};
	});

	// The painting's too-flat rules (12/5, and under the sheet 192px: the
	// container queries in the style block) hide it; in the frame the plate
	// then collapses too, so the sheet takes the frame instead of leaving an
	// empty band of paper. Worked out from what the plate would be, not what
	// it is, or a collapsed plate would stay collapsed: the plate is the frame
	// less the sheet's contents, at most 106vw, and the sheet lies 24px over
	// it. The contents count the heading as its text alone, without the
	// header a collapsed plate gives it (the mascot, the cover's lead), so
	// the header never decides its own collapse.
	//
	// The sheet never restructures while typing: its contents are the same
	// whatever the keyboard's height, and a sheet taller than the frame
	// scrolls inside. A plate collapsed while a field has focus stays
	// collapsed until the field loses it, so the keyboard growing or
	// shrinking by its number row (Gboard's, in a password) only moves the
	// sheet. It comes back once no field has focus and it fits again.
	const OVERLAP = 24;
	const FLAT = 12 / 5;
	const SHEET_LOW = 192;
	let flat = $state(false);

	const heading = () => card.querySelector('h1');

	function typing() {
		const el = document.activeElement;
		return isTextField(el) && sheet.contains(el);
	}

	// Called outside Svelte's flush only (a ResizeObserver, a microtask, a
	// timer): `flushSync` applies `flat` at once, so the move that follows
	// reads the new layout before the paint.
	function measure() {
		let contents = contentHeight(sheet);
		const h1 = heading();
		if (h1) {
			const text = document.createRange();
			text.selectNodeContents(h1);
			contents += text.getBoundingClientRect().height - h1.offsetHeight;
		}
		const width = main.clientWidth;
		const height = main.clientHeight;
		const up = main.hasAttribute('data-sheet');
		const plateHeight = Math.min(1.06 * width, height - contents + (up ? OVERLAP : 0));
		const tooFlat =
			plateHeight <= 0 || width / plateHeight > FLAT || (up && plateHeight < SHEET_LOW);
		flat = tooFlat || (flat && typing());
		flushSync();
	}
	$effect(() => {
		if (!framed) return;
		const observer = new ResizeObserver(follow);
		observer.observe(main);
		observer.observe(card);
		// The cover's contents change too (a provider button arriving late).
		if (coverParts) observer.observe(coverParts);
		// A field losing focus releases a collapse held while typing, even if
		// nothing resizes after it (a hardware keyboard); on the next tick,
		// once focus has landed wherever it goes next.
		let timer: ReturnType<typeof setTimeout> | undefined;
		const release = () => {
			clearTimeout(timer);
			timer = setTimeout(() => follow());
		};
		document.addEventListener('focusout', release);
		return () => {
			document.removeEventListener('focusout', release);
			clearTimeout(timer);
			observer.disconnect();
			stop();
			undim();
			last = null;
			flat = false;
		};
	});

	// Motion inside the frame. The frame itself follows the visible area at
	// once, so the sheet never ends under the keyboard; inside it the plate,
	// the sheet and the card move from where they are drawn to the new layout
	// (FLIP), on the cover's rise (420ms, the sheet up from the frame's bottom
	// edge) and whenever the frame or the contents change while the sheet is
	// up or something still moves (260ms: the keyboard coming or going, the
	// error line). A move that is still running is taken over from where it
	// is, never restarted from the old layout, so a burst of resizes reads as
	// one motion. The painting fades as the plate collapses or comes back,
	// and the heading's header grows in or shrinks away with it (the gap, the
	// mascot's size, the lead's height and their opacity). Nothing moves on
	// arrival.
	type Head = { gap: number; size: number; lead: number; shown: number };
	type Shown = { plate: number; top: number; image: number; card?: number; head?: Head | null };
	let plate: HTMLElement;
	let image: HTMLImageElement;
	// The layout the last move ends in, which is also what is drawn once it
	// has ended.
	let last: Shown | null = null;
	let moves: Animation[] = [];
	let from: Shown = { plate: 0, top: 0, image: 1 };
	let arrived = false;

	// The rise's dims, apart from the moves: a move during the rise (the
	// keyboard coming up with the sheet) takes the moves over, and the dims
	// would snap to full with them. Only leaving and closing cancel them.
	let dims: Animation[] = [];

	// Paused counts: a test holding a frame still must not start a new move.
	const active = (move: Animation) => move.playState === 'running' || move.playState === 'paused';
	const running = () => [...moves, ...dims].filter(active);

	function undim() {
		for (const dim of dims) dim.cancel();
		dims = [];
	}
	const translateY = (el: Element) =>
		parseFloat(getComputedStyle(el).translate.split(' ')[1] ?? '') || 0;

	// The heading's header as drawn: none, whole, or on its way.
	function head(): Head | null {
		const h1 = heading();
		if (!h1) return null;
		const mascot = getComputedStyle(h1, '::before');
		return {
			gap: parseFloat(getComputedStyle(h1).columnGap) || 0,
			size: parseFloat(mascot.width) || 0,
			lead: parseFloat(getComputedStyle(h1, '::after').height) || 0,
			shown: parseFloat(mascot.opacity) || 0
		};
	}

	// What the layout is now, with nothing moving.
	const layout = (): Shown & { card: number } => ({
		plate: plate.offsetHeight,
		top: sheet.offsetTop,
		image: flat ? 0 : 1,
		card: card.offsetTop,
		head: head()
	});

	// Where the plate, the sheet's top, the card, the painting and the
	// header are drawn right now.
	function shown(): Shown | null {
		if (!running().length) return last;
		return {
			plate: plate.getBoundingClientRect().height,
			top: sheet.offsetTop + translateY(sheet),
			image: parseFloat(getComputedStyle(image).opacity),
			card: card.offsetTop + translateY(card),
			head: head()
		};
	}

	function stop() {
		for (const move of moves) move.cancel();
		moves = [];
	}

	const glide = () => getComputedStyle(main).getPropertyValue('--ease-glide').trim() || 'ease-out';

	// From what is drawn (`start`) to the layout as it is now.
	function move(start: Shown, duration: number) {
		const left = Math.max(
			0,
			...running().map((m) => Number(m.effect?.getTiming().duration) - Number(m.currentTime))
		);
		stop();
		const end = layout();
		last = end;
		if (prefersReducedMotion.current) return;
		const timing = { duration: Math.max(duration, left), easing: glide() };
		if (start.plate !== end.plate) {
			moves.push(
				plate.animate(
					{ flexBasis: [`${start.plate}px`, `${end.plate}px`], flexShrink: [0, 0] },
					timing
				)
			);
		}
		// The sheet's top follows the plate's edge as it moves; what the plate
		// does not account for is a translate that runs out with it.
		const shift = start.top - start.plate - (end.top - end.plate);
		if (shift) moves.push(sheet.animate({ translate: [`0 ${shift}px`, '0 0'] }, timing));
		// The card, centered in the sheet, from where it was in it, when only
		// the frame moved it (the keyboard changing height over a collapsed
		// plate). With the plate moving, the sheet's height moves with it and
		// carries the card along by itself.
		if (start.card !== undefined && start.plate === end.plate && start.card !== end.card) {
			moves.push(card.animate({ translate: [`0 ${start.card - end.card}px`, '0 0'] }, timing));
		}
		if (start.image !== end.image) {
			moves.push(image.animate({ opacity: [start.image, end.image] }, timing));
		}
		const h1 = heading();
		const a = start.head;
		const b = end.head;
		if (h1 && a && b && (a.size !== b.size || a.lead !== b.lead)) {
			const px = (n: number) => `${n}px`;
			moves.push(
				h1.animate({ columnGap: [px(a.gap), px(b.gap)] }, timing),
				h1.animate(
					{
						width: [px(a.size), px(b.size)],
						height: [px(a.size), px(b.size)],
						opacity: [a.shown, b.shown]
					},
					{ ...timing, pseudoElement: '::before' }
				),
				h1.animate(
					{ height: [px(a.lead), px(b.lead)], opacity: [a.shown, b.shown] },
					{ ...timing, pseudoElement: '::after' }
				)
			);
			// The card resizes on every frame of it, which this ignores (see
			// follow); once it ends, the layout is checked once more, and the
			// focused field, which the header may have pushed down, revealed.
			moves.at(-1)?.finished.then(
				() => {
					follow();
					void reveal();
				},
				() => {}
			);
		}
	}

	const resizing = () => heading()?.getAnimations({ subtree: true }).some(active) ?? false;

	// The layout the moves under way end in, read without restarting them:
	// each is seeked to its end for the reading (where it no longer applies)
	// and back. Read as drawn instead, a resize while one ran never matched
	// `last`, and the move began again on a fresh curve.
	function target() {
		const held = running().map((move) => [move, move.currentTime] as const);
		for (const [move] of held) move.currentTime = Number(move.effect?.getTiming().duration);
		const end = layout();
		for (const [move, time] of held) move.currentTime = time;
		return end;
	}

	// The ResizeObserver's callback: the frame or the contents changed.
	function follow(entries?: ResizeObserverEntry[]) {
		if (still) return;
		// The header's own motion resizes the card; that is no new layout.
		if (entries?.every((entry) => entry.target === card) && resizing()) return;
		const start = shown();
		measure();
		const end = target();
		const same =
			last &&
			end.plate === last.plate &&
			end.top === last.top &&
			end.image === last.image &&
			end.card === last.card;
		if (same) return;
		if (!start || (!main.hasAttribute('data-sheet') && !running().length)) {
			stop();
			last = end;
			return;
		}
		move(start, 260);
	}

	$effect.pre(() => {
		void open;
		untrack(() => {
			if (!plate) return;
			from = shown() ?? {
				plate: plate.getBoundingClientRect().height,
				top: 0,
				image: flat ? 0 : 1
			};
		});
	});
	$effect(() => {
		void open;
		untrack(() => {
			if (arrived && framed && frame === 'cover') queueMicrotask(rise);
			arrived = true;
		});
	});

	function rise() {
		// Queued: the page may have gone in the meantime.
		if (!main?.isConnected) return;
		measure();
		// Up from the frame's bottom edge; closing, the cover's paper simply
		// follows the plate.
		move(
			{ ...from, top: open ? main.clientHeight : from.plate, card: undefined, head: undefined },
			420
		);
		if (!open || prefersReducedMotion.current) return;
		// The dim behind the sheet (the plate's and the frame's own) fades in with it.
		const timing = { duration: 420, easing: glide() };
		undim();
		dims.push(
			plate.animate({ opacity: [0, 1] }, { ...timing, pseudoElement: '::after' }),
			main.animate({ opacity: [0, 1] }, { ...timing, pseudoElement: '::before' })
		);
	}

	// Back to the cover, its first button focused: a tap on the plate,
	// Escape, the grip (a tap or a pull) and the platform's close request.
	// A sheet pulled past the threshold carries on from where the finger let
	// go down to the frame's bottom edge first, rather than snapping back.
	let closing = false;
	function close() {
		if (!open || closing) return;
		const pulled = new DOMMatrix(getComputedStyle(sheet).transform).m42;
		if (pulled <= 0 || prefersReducedMotion.current) {
			settle();
			return;
		}
		closing = true;
		const gone = `translateY(${main.clientHeight - sheet.offsetTop}px)`;
		sheet.style.transform = gone;
		sheet
			.animate({ transform: [`translateY(${pulled}px)`, gone] }, { duration: 200, easing: glide() })
			.finished.then(settle, settle);
	}

	function settle() {
		closing = false;
		undim();
		if (!sheet?.isConnected) return;
		sheet.style.transform = '';
		open = false;
		flushSync();
		main.querySelector<HTMLElement>('[data-auth-cover] button')?.focus();
	}

	function closeOnEscape(event: KeyboardEvent) {
		if (event.key !== 'Escape' || !dialog) return;
		event.preventDefault();
		close();
	}

	$effect(() => {
		const Watcher = (window as CloseWatchers).CloseWatcher;
		if (!dialog || !Watcher) return;
		const watcher = new Watcher();
		watcher.onclose = close;
		return () => watcher.destroy();
	});

	// What the sheet's contents need, whatever height the sheet has: its
	// padding and its shown children with the gaps between them. Not its
	// scrollHeight, which is its own height once it is taller than them.
	function contentHeight(el: HTMLElement): number {
		const style = getComputedStyle(el);
		const shown = [...el.children].filter((child) => child.getClientRects().length > 0);
		const children = shown.reduce((sum, child) => sum + (child as HTMLElement).offsetHeight, 0);
		return (
			parseFloat(style.paddingTop) +
			parseFloat(style.paddingBottom) +
			children +
			Math.max(0, shown.length - 1) * parseFloat(style.rowGap)
		);
	}

	// Nothing moves as the page opens: the field focused on arrival, a value
	// filled in from the last page and the card lifted for it are simply
	// there. The first press of a key or a pointer lets the card and its
	// fields answer from then on (`data-ready`).
	let ready = $state(false);
	const arm = () => (ready = true);

	let card: HTMLElement;

	/** Shakes the card, the answer to a sign-in the service refused. */
	export function shake() {
		if (prefersReducedMotion.current) {
			return;
		}
		// `transform`, not `translate`: the lift while a field has focus is a
		// `translate`, and the two compose.
		card.animate(
			{ transform: [0, -9, 8, -5, 3, -1, 0].map((x) => `translateX(${x}px)`) },
			{ duration: 460, easing: 'cubic-bezier(0.36, 0.07, 0.19, 0.97)' }
		);
	}
</script>

<svelte:window onkeydowncapture={arm} onpointerdowncapture={arm} onkeydown={closeOnEscape} />

<!-- Two layouts from one tree. From lg the kitchen scene covers the window and
     the card lies on its empty right third. Below lg the page opens like a
     cookbook: the scene as a plate on top, the fold's shadow, then a text
     page with the wordmark, a printer's ornament and the card's contents. -->
{#if framed}
	<span bind:this={probe} aria-hidden="true" class="pointer-events-none fixed inset-y-0 left-0 w-0"
	></span>
{/if}
<main
	bind:this={main}
	data-ready={ready || undefined}
	data-frame={frame}
	data-sheet={frame === 'sheet' || (frame === 'cover' && open) || undefined}
	data-flat={flat || undefined}
	style:top={framed ? `${area.top}px` : undefined}
	style:bottom={framed ? `${area.bottom}px` : undefined}
	style:--auth-mascot={framed ? `url("${mascot}")` : undefined}
	style:--auth-lead={framed && frame === 'cover' && lead ? JSON.stringify(lead) : undefined}
	class="relative flex min-h-screen flex-col bg-surface lg:block lg:bg-transparent"
>
	<!-- Below lg the plate frames the painting on the mascot (see the style
	     block); from lg both wrappers step aside (`contents`) and the image
	     covers the window as before. -->
	<!-- A tap on the painting closes the sheet: a pointer's way out; Escape
	     and the grip are the keys'. -->
	<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
	<div bind:this={plate} data-auth-plate class="lg:contents" onclick={dialog ? close : undefined}>
		<div data-auth-view inert={dialog} class="lg:contents">
			<img
				bind:this={image}
				src={kitchen}
				alt=""
				class="lg:absolute lg:inset-0 lg:h-full lg:w-full lg:object-cover lg:object-[52%_100%]"
			/>
		</div>
	</div>
	<!-- `data-auth-page` and `data-auth-card`: the text page below lg and the
	     card from lg. -->
	<div
		bind:this={sheet}
		data-auth-page
		role={dialog ? 'dialog' : undefined}
		aria-modal={dialog || undefined}
		aria-labelledby={dialog ? labelledby : undefined}
		class="relative flex flex-1 flex-col items-center gap-3.5 px-6 py-8 before:absolute before:inset-x-0 before:top-0 before:h-[22px] before:bg-linear-to-b before:from-overlay/50 before:to-transparent lg:ml-auto lg:min-h-screen lg:w-[460px] lg:items-stretch lg:justify-center lg:gap-5 lg:px-12 lg:py-14 lg:before:hidden dark:before:from-lightbox/70"
	>
		{#if dialog}
			<button
				type="button"
				aria-label={m.common_close()}
				onclick={close}
				{@attach pullToClose(close)}
				class="flex h-11 shrink-0 touch-none items-center justify-center self-stretch focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
			>
				<span class="h-1 w-9 rounded-pill bg-handle" aria-hidden="true"></span>
			</button>
		{/if}
		<!-- The wordmark as a mask, so on the text page it can take the theme's
		     ink; on the painting it keeps its own. Quoted: Vite inlines the small
		     SVG as a data URI, whose parentheses break a bare url(). -->
		<span
			data-cover
			role="img"
			aria-label={m.app_name()}
			style:mask-image={`url("${wordmark}")`}
			class="block aspect-[386.8/94.6] w-[200px] bg-accent-foreground mask-contain mask-center mask-no-repeat forced-color-adjust-none lg:w-[250px] lg:self-center lg:bg-scene-ink"
		></span>
		<div
			data-cover
			aria-hidden="true"
			class="flex w-[180px] items-center gap-3 text-body text-primary before:h-px before:flex-1 before:bg-border after:h-px after:flex-1 after:bg-border lg:hidden"
		>
			❦
		</div>
		{#if lead}
			<p
				data-cover
				class="mb-1.5 text-center font-display text-body-lg text-balance text-text-muted italic lg:-mt-2.5 lg:mb-0 lg:font-sans lg:text-scene-muted lg:not-italic"
			>
				{lead}
			</p>
		{/if}
		{#if cover && framed && frame === 'cover'}
			<div
				bind:this={coverParts}
				data-cover
				data-auth-cover
				class="flex w-full max-w-[440px] flex-col gap-3"
			>
				{@render cover()}
			</div>
		{/if}
		<!-- empty:hidden: a page still loading passes nothing yet, and an empty
		     card would flash on the painting. While a field has focus the card
		     is picked up off the table: it lifts on a soft spring and its
		     shadow deepens. -->
		<div
			bind:this={card}
			data-auth-card
			class="flex w-full max-w-[440px] flex-col gap-5 empty:hidden in-data-ready:transition-[translate,box-shadow] in-data-ready:duration-520 in-data-ready:ease-spring lg:rounded-2xl lg:bg-surface lg:p-6 lg:shadow-lift lg:focus-within:shadow-dialog motion-safe:lg:focus-within:-translate-y-1.25 dark:lg:border dark:lg:border-border"
		>
			{@render children()}
		</div>
	</div>
</main>

<style>
	@media (width < 64rem) {
		[data-auth-plate] {
			/* Properties of the painting, rezepte-kitchen.webp (2560x1086): its
			   width over its height, and the mascot's center as a fraction of its
			   width. The mascot spans x 21.8%-53.8% and y 11.2%-84.5% of it; the
			   12/5 and the 192px below and the e2e figure box come from that
			   span. A new painting changes them here. */
			--painting-ratio: 2.357;
			--figure-center: 0.378;
			--figure-foot: 0.845;
			position: relative;
			overflow: hidden;
			container-type: size;
			height: max(250px, 45vw);
			flex-shrink: 0;
		}

		/* The painting's box, the whole plate: in sheet state the painting
		   goes on under the sheet's 24px overlap, so the sheet lies over it. */
		[data-auth-view] {
			position: absolute;
			inset: 0;
			container: view / size;
		}

		/* Bottom-anchored, so the table edge stays; as wide as needed to fill
		   the view, the mascot's center on the view's center, clamped so no
		   empty edge shows. */
		img {
			--w: max(100cqw, calc(var(--painting-ratio) * 100cqh));
			position: absolute;
			bottom: 0;
			width: var(--w);
			max-width: none;
			height: auto;
			left: clamp(100cqw - var(--w), 50cqw - var(--figure-center) * var(--w), 0px);
		}

		/* Under the sheet at least so tall that the floor below the feet
		   reaches 8px above the sheet's top: (24px + 8px) over the painting's
		   share below the feet, about 206px. */
		main[data-sheet] img {
			--w: max(
				100cqw,
				calc(var(--painting-ratio) * 100cqh),
				calc(var(--painting-ratio) * 32px / (1 - var(--figure-foot)))
			);
		}

		/* Too flat for the whole mascot with 8px over its head: no painting,
		   the paper shows instead. 12/5 holds that margin from 320 to 440px.
		   In the frame `measure()` decides it instead (`data-flat`, with the
		   192px under the sheet: the painting there is at least 206px tall,
		   whose figure takes 192px of the plate with its 8px), and the
		   painting fades rather than dropping out in one frame. */
		@container view (aspect-ratio > 12/5) {
			main:not([data-frame]) img {
				display: none;
			}
		}

		main[data-frame][data-flat] img {
			opacity: 0;
		}

		/* The frame: the visible area (top and bottom set inline from
		   followVisibleArea; without them the whole window), nothing in it
		   pinned, so the keyboard shrinks it and never covers it. */
		main[data-frame] {
			position: fixed;
			inset: 0;
			min-height: 0;
			/* clip, not hidden: a field scrolled into view (reveal) while the
			   sheet is still below the frame would scroll a hidden frame. */
			overflow: clip;
		}

		/* The plate gives way first, down to nothing: the sheet keeps its
		   height up to the frame's and scrolls inside past that. */
		main[data-frame] [data-auth-plate] {
			flex: 0 1 106vw;
			height: auto;
			min-height: 0;
		}

		main[data-flat] [data-auth-plate] {
			flex-basis: 0;
		}

		/* Dimmed: the sheet is what is in front. The frame behind the plate is
		   dimmed too, for what shows as the cover's sheet rises or is pulled
		   down; under the plate the opaque painting hides it, so the two dims
		   never stack. */
		main[data-frame][data-sheet]::before {
			content: '';
			position: absolute;
			inset: 0;
			background: color-mix(in oklab, var(--color-overlay) 50%, transparent);
		}

		/* Dimmed: the sheet is what is in front, also in the wedges beside its
		   rounded corners, where the painting shows on under it. */
		main[data-sheet] [data-auth-plate]::after {
			content: '';
			position: absolute;
			inset: 0;
			background: color-mix(in oklab, var(--color-overlay) 50%, transparent);
		}

		/* The paper page as the sheet, lying 24px over the plate and reaching
		   the frame's bottom. It never shrinks for the plate, only stops at
		   the frame's height and scrolls inside. */
		main[data-sheet] [data-auth-page] {
			flex: 1 0 auto;
			max-height: 100%;
			margin-top: -24px;
			overflow-y: auto;
			overscroll-behavior: contain;
			border-radius: var(--radius-3xl) var(--radius-3xl) 0 0;
			background: var(--color-surface);
			box-shadow: var(--shadow-sheet);
		}

		/* Paper left over splits above and below the card; auto margins drop
		   to 0 once the sheet scrolls, so its top is never cut off. */
		main[data-sheet] [data-auth-card] {
			margin-block: auto;
		}

		/* With the plate gone the sheet is the page: no corners at the top. */
		main[data-flat] [data-auth-page] {
			margin-top: 0;
			border-radius: 0;
		}

		/* The cover: wordmark, ornament and lead centered in the paper between
		   the plate and the cover's buttons, which stand at its foot; the card
		   waits, not displayed, until the sheet is up. */
		main[data-frame='cover']:not([data-sheet]) [data-auth-page] {
			padding-bottom: calc(1.5rem + env(safe-area-inset-bottom));
		}

		main[data-frame='cover']:not([data-sheet]) [data-auth-page] > [data-cover]:first-child,
		main[data-frame='cover']:not([data-sheet]) [data-auth-cover] {
			margin-top: auto;
		}

		main[data-frame='cover']:not([data-sheet]) [data-auth-card] {
			display: none;
		}

		/* The cover's sheet starts with its grip, which brings its own room. */
		main[data-frame='cover'][data-sheet] [data-auth-page] {
			padding-top: 0;
		}

		main[data-sheet] [data-auth-page]::before,
		main[data-sheet] [data-cover] {
			display: none;
		}

		/* In the sheet the card's heading shows, also where the page keeps it
		   for screen readers only (the login's sr-only). With the painting
		   gone it becomes a small header: the mascot (a decorative
		   background, --auth-mascot) beside it, and on the cover's sheet the
		   cover's lead (--auth-lead, a CSS string) under it, its alt text
		   empty so the dialog's name stays the heading. Laid out in every
		   sheet, at no size without the collapse, so the header can grow in
		   and shrink away (move in the script). */
		main[data-sheet] [data-auth-card] :global(h1) {
			position: static;
			width: auto;
			height: auto;
			margin: 0;
			overflow: visible;
			clip-path: none;
			white-space: normal;
			display: grid;
			grid-template-columns: auto 1fr;
			column-gap: 0;
			align-items: center;
		}

		main[data-sheet] [data-auth-card] :global(h1)::before {
			content: '';
			grid-row: 1 / span 2;
			width: 0;
			height: 0;
			opacity: 0;
			background: var(--auth-mascot) center / contain no-repeat;
			filter: drop-shadow(var(--drop-shadow-figure));
		}

		main[data-frame='cover'][data-sheet] [data-auth-card] :global(h1)::after {
			content: var(--auth-lead) / '';
			height: 0;
			overflow: hidden;
			opacity: 0;
			font-family: var(--font-display);
			font-size: var(--text-body);
			font-style: italic;
			font-weight: normal;
			line-height: 1.3;
			color: var(--color-text-muted);
			align-self: start;
		}

		main[data-flat][data-sheet] [data-auth-card] :global(h1) {
			column-gap: 12px;
		}

		main[data-flat][data-sheet] [data-auth-card] :global(h1)::before {
			width: 56px;
			height: 56px;
			opacity: 1;
		}

		main[data-frame='cover'][data-flat][data-sheet] [data-auth-card] :global(h1)::after {
			height: auto;
			opacity: 1;
		}
	}
</style>
