<script lang="ts" module>
	import { prefersReducedMotion } from 'svelte/motion';

	/** How far the spotlight's hole stands out from the step, on every side. */
	const SPOTLIGHT_MARGIN = 10;

	/**
	 * Where the screen stops being visible at the top: the bottom edge of the
	 * bars pinned there (`data-covers-top`: the phone's `EditorBar`, the
	 * detail page's top bar, a running head), or 0 where none is. A value
	 * caps the bar's cover at that many rem: the detail page's bar counts as
	 * filled in even before a scroll fills it, as a reveal's own scroll may.
	 */
	export function coveredTop(): number {
		const rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
		const bars = [...document.querySelectorAll<HTMLElement>('[data-covers-top]')]
			.map((bar) => {
				const box = bar.getBoundingClientRect();
				const cap = parseFloat(bar.dataset.coversTop ?? '') * rem;
				return { top: box.top, bottom: cap ? Math.min(box.bottom, box.top + cap) : box.bottom };
			})
			.filter((box) => box.bottom > box.top)
			.sort((a, b) => a.top - b.top);
		let edge = 0;
		// Only a bar that starts where the one above it ends is pinned there.
		for (const box of bars) if (box.top <= edge + 1) edge = Math.max(edge, box.bottom);
		return edge;
	}

	/** The scroll box `element` sits in below the page, if any: cook mode's step. */
	export function scrollBox(element: HTMLElement): HTMLElement | null {
		for (let at = element.parentElement; at && at !== document.body; at = at.parentElement) {
			if (/auto|scroll/.test(getComputedStyle(at).overflowY)) return at;
		}
		return null;
	}

	/** What `revealAbove` padded at its bottom, cleared again when the sheet closes. */
	let padded: HTMLElement | null = null;

	/**
	 * Keeps a phone sheet's subject in sight above it: the lines `lines`
	 * measures of `anchor` (its first line's top, its last line's bottom)
	 * scroll until the last ends `gap` above the sheet's resting top - the
	 * sheet is still flying in. Never so far that the first line goes under
	 * the bars pinned to the top (`coveredTop`) or past its scroll box's top:
	 * a step too tall for the room shows the phrase's lines and lets its own
	 * top go behind the bar. The page scrolls, padded at its end when it is
	 * too short to, unless `anchor` sits in a scroll box of its own (cook
	 * mode, whose step is centered and rarely overflows): that box is padded
	 * by what the sheet covers of it, which re-centers the step above the
	 * sheet or lets it scroll there. The padding goes when the sheet closes.
	 */
	export function revealAbove(
		sheet: HTMLElement,
		anchor: HTMLElement,
		lines: () => { top: number; bottom: number },
		gap = 12
	) {
		const viewport = window.visualViewport;
		const bottom = viewport ? viewport.offsetTop + viewport.height : window.innerHeight;
		const top = bottom - sheet.offsetHeight - gap;
		const scroller = scrollBox(anchor);
		if (scroller) {
			const covered = scroller.getBoundingClientRect().bottom - top;
			if (covered > 0) pad(scroller, covered, false);
		}
		const { top: first, bottom: last } = lines();
		const ceiling = scroller ? scroller.getBoundingClientRect().top : coveredTop() + gap;
		const below = Math.min(last - top, first - ceiling);
		if (below <= 0) return;
		const behavior = prefersReducedMotion.current ? 'auto' : 'smooth';
		if (scroller) {
			scroller.scrollBy({ top: below, behavior });
			return;
		}
		// A step near the end of a short page: the page grows by what it lacks.
		const room = document.documentElement.scrollHeight - window.innerHeight - window.scrollY;
		if (below > room) pad(document.body, below - room, true);
		window.scrollBy({ top: below, behavior });
	}

	/** Sets `element`'s bottom padding to `px`, or adds `px` to what it has. */
	function pad(element: HTMLElement, px: number, add: boolean) {
		const has = add ? parseFloat(getComputedStyle(element).paddingBottom) || 0 : 0;
		element.style.paddingBottom = `${has + px}px`;
		padded = element;
	}
</script>

<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { tick, type Snippet } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { followVisibleArea, type VisibleArea } from '#lib/visible-area.js';
	import SheetHandle from './SheetHandle.svelte';

	// Shared Overlay + Content wrapper for the phone's bottom sheets (the
	// contents, You, one person), so the phone has one kind of sheet: a
	// rounded panel rising from the bottom edge, its handle closing it on a
	// tap or a pull.
	// The caller renders the title and the scrolling body inside it.
	let {
		open = $bindable(false),
		closeLabel,
		onOpenAutoFocus,
		onCloseAutoFocus,
		onInteractOutside,
		tall = false,
		spotlight = null,
		children
	}: {
		open?: boolean;
		/** Accessible name of the handle, which closes the sheet. */
		closeLabel: string;
		/** Forwarded to Bits UI: where focus goes once the sheet has opened. */
		onOpenAutoFocus?: (event: Event) => void;
		/** Forwarded to Bits UI: where focus goes once the sheet has closed. */
		onCloseAutoFocus?: (event: Event) => void;
		/** Forwarded to Bits UI: a press outside the sheet, prevented to keep it open. */
		onInteractOutside?: (event: Event) => void;
		/**
		 * Up to 92dvh instead of 80dvh, for a sheet whose content is the
		 * point of opening it (the pass-on sheet): Safari's bars leave an
		 * iPhone about 660px, and 80dvh of that dropped the pass-on ticket.
		 */
		tall?: boolean;
		/**
		 * An element the dim leaves out: it stays as bright as the sheet and
		 * takes taps, for a sheet about something on the page, such as a
		 * step's word. A click there is outside the sheet for Bits UI and
		 * closes it; a caller that wants a tap there to do more answers it
		 * first (the step editor at `touchend`). The elements marked
		 * `data-spotlight-with` in its nearest `data-spotlight-scope` - a
		 * step's number pill and x, which stick out over its top border - are
		 * left out with it, the hole standing clear of all of them.
		 */
		spotlight?: HTMLElement | null;
		children: Snippet;
	} = $props();

	// The sheet rests on the software keyboard ([Sheets] in the design
	// system): on iOS, where the keyboard only shrinks the visual viewport,
	// `followVisibleArea` lifts it by the keyboard's height and caps it to the
	// room above; lifted off the screen edge it is a card, so it rounds its
	// bottom corners too. Chromium shrinks the layout itself and reads 0.
	// Every time the area moves, the focused field scrolls back into view
	// inside the sheet's body.
	let area = $state<VisibleArea>({ top: 0, bottom: 0 });
	let probe = $state<HTMLElement>();
	$effect(() => {
		const line = probe;
		if (!open || !line) return;
		const stop = followVisibleArea(line, (next) => {
			area = next;
			void reveal();
		});
		return () => {
			stop();
			area = { top: 0, bottom: 0 };
		};
	});
	async function reveal() {
		await tick();
		const focused = document.activeElement;
		if (!(focused instanceof HTMLElement) || !panel?.contains(focused)) return;
		// A field grown taller than the body would be scrolled to its top,
		// taking the caret out of sight; the browser keeps the caret in view.
		const body = focused.closest('[data-sheet-body]');
		if (!body || focused.offsetHeight <= body.clientHeight) {
			focused.scrollIntoView({ block: 'nearest' });
		}
	}
	const lifted = $derived(area.bottom > 0);

	// A sheet that scrolls shows it all the time: a phone hides its
	// scrollbar until a swipe, and a sheet cut off by the screen's edge then
	// reads as broken. The body that scrolls (the caller's
	// `data-sheet-body`) gets a thumb of its own, drawn over its right edge,
	// in place of the browser's (hidden in app.css).
	let panel = $state<HTMLElement>();
	let thumb = $state<{ top: number; height: number } | null>(null);
	$effect(() => {
		const body = panel?.querySelector<HTMLElement>('[data-sheet-body]');
		if (!body) return;
		const measure = () => {
			const { scrollTop, scrollHeight, clientHeight } = body;
			if (scrollHeight <= clientHeight + 1) {
				thumb = null;
				return;
			}
			const track = clientHeight - 8;
			const height = Math.max(24, (track * clientHeight) / scrollHeight);
			const share = scrollTop / (scrollHeight - clientHeight);
			thumb = { top: body.offsetTop + 4 + (track - height) * share, height };
		};
		const sizes = new ResizeObserver(measure);
		const watch = () => {
			sizes.observe(body);
			for (const child of body.children) sizes.observe(child);
		};
		// Content that comes and goes (an error line, a field that unfolds)
		// changes the height to scroll without resizing the body.
		const changes = new MutationObserver(() => {
			watch();
			measure();
		});
		watch();
		changes.observe(body, { childList: true, subtree: true });
		body.addEventListener('scroll', measure, { passive: true });
		measure();
		return () => {
			sizes.disconnect();
			changes.disconnect();
			body.removeEventListener('scroll', measure);
			thumb = null;
		};
	});

	const duration = $derived(prefersReducedMotion.current ? 0 : 250);
	const fadeDuration = $derived(prefersReducedMotion.current ? 0 : 200);

	/**
	 * The dim with a hole where `spotlight` is: a clip path, so the hole is
	 * clipped out of the hit test too and a tap there reaches the element.
	 * The hole takes in its `data-spotlight-with` elements, stands
	 * `SPOTLIGHT_MARGIN` out from all of them and rounds its corners
	 * concentric with the spotlight's. It shows only what shows of the
	 * spotlight: cut flat, square-cornered, at the edges of a box that clips
	 * it (cook mode's step) and at the bars pinned to the top
	 * (`coveredTop`), so a step scrolled under the editor's bar stays behind
	 * the bar. Followed on scroll and resize, since the page under the sheet
	 * scrolls.
	 */
	let hole = $state<string | undefined>(undefined);
	function measure() {
		if (!spotlight) {
			hole = undefined;
			return;
		}
		const scope = spotlight.closest('[data-spotlight-scope]');
		const boxes = [spotlight, ...(scope?.querySelectorAll('[data-spotlight-with]') ?? [])].map(
			(element) => element.getBoundingClientRect()
		);
		const out = {
			left: Math.min(...boxes.map((box) => box.left)) - SPOTLIGHT_MARGIN,
			top: Math.min(...boxes.map((box) => box.top)) - SPOTLIGHT_MARGIN,
			right: Math.max(...boxes.map((box) => box.right)) + SPOTLIGHT_MARGIN,
			bottom: Math.max(...boxes.map((box) => box.bottom)) + SPOTLIGHT_MARGIN
		};
		const view = { left: -1e5, top: coveredTop(), right: 1e5, bottom: 1e5 };
		for (let at = spotlight.parentElement; at && at !== document.body; at = at.parentElement) {
			const style = getComputedStyle(at);
			const box = at.getBoundingClientRect();
			if (style.overflowX !== 'visible') {
				view.left = Math.max(view.left, box.left);
				view.right = Math.min(view.right, box.right);
			}
			if (style.overflowY !== 'visible') {
				view.top = Math.max(view.top, box.top);
				view.bottom = Math.min(view.bottom, box.bottom);
			}
		}
		const left = Math.max(out.left, view.left);
		const top = Math.max(out.top, view.top);
		const right = Math.min(out.right, view.right);
		const bottom = Math.min(out.bottom, view.bottom);
		// The outer edge reaches far past any screen: a zoomed page's
		// innerWidth is smaller than the dim, which would then cover only its
		// top left.
		const dim = 'M-1e5 -1e5H1e5V1e5H-1e5Z';
		if (right <= left || bottom <= top) {
			hole = `path("${dim}")`;
			return;
		}
		const k = Math.min(
			(parseFloat(getComputedStyle(spotlight).borderTopLeftRadius) || 0) + SPOTLIGHT_MARGIN,
			(bottom - top) / 2,
			(right - left) / 2
		);
		// A corner on a cut edge is square: the step goes on past it.
		const r = (x: boolean, y: boolean) => (x || y ? 0 : k);
		const tl = r(left > out.left, top > out.top);
		const tr = r(right < out.right, top > out.top);
		const br = r(right < out.right, bottom < out.bottom);
		const bl = r(left > out.left, bottom < out.bottom);
		const arc = (k: number, x: number, y: number) => `A${k} ${k} 0 0 0 ${x} ${y}`;
		// Drawn the other way round from the dim, so it is a hole under `nonzero`.
		hole =
			`path("${dim} M${right - tr} ${top}H${left + tl}${arc(tl, left, top + tl)}` +
			`V${bottom - bl}${arc(bl, left + bl, bottom)}H${right - br}${arc(br, right, bottom - br)}` +
			`V${top + tr}${arc(tr, right - tr, top)}Z")`;
	}
	// Whatever `revealAbove` padded while this sheet was open goes with it.
	$effect(() => {
		if (!open) return;
		return () => {
			if (padded) padded.style.paddingBottom = '';
			padded = null;
		};
	});

	$effect(() => {
		if (!open || !spotlight) return;
		measure();
		window.addEventListener('scroll', measure, true);
		window.addEventListener('resize', measure);
		return () => {
			window.removeEventListener('scroll', measure, true);
			window.removeEventListener('resize', measure);
		};
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Portal>
		{#if open}
			<span
				bind:this={probe}
				aria-hidden="true"
				class="pointer-events-none fixed inset-y-0 left-0 w-0"
			></span>
		{/if}
		<Dialog.Overlay forceMount>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed inset-0 z-40 bg-overlay"
						style:clip-path={hole}
						transition:fade={{ duration: fadeDuration }}
					></div>
				{/if}
			{/snippet}
		</Dialog.Overlay>
		<Dialog.Content
			forceMount
			preventScroll={false}
			{onOpenAutoFocus}
			{onCloseAutoFocus}
			{onInteractOutside}
		>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						bind:this={panel}
						class="fixed inset-x-0 bottom-0 z-50 mx-auto flex {tall
							? 'max-h-[92dvh]'
							: 'max-h-[80dvh]'} w-full max-w-[640px] flex-col {lifted
							? 'rounded-3xl'
							: 'rounded-t-3xl'} bg-background shadow-sheet outline-none"
						style:bottom={lifted ? `${area.bottom}px` : undefined}
						style:max-height={lifted
							? `calc(100dvh - ${area.top + area.bottom + 16}px)`
							: undefined}
						transition:fly={{ duration, y: 200 }}
					>
						<SheetHandle label={closeLabel} onclose={() => (open = false)} />
						{@render children()}
						{#if thumb}
							<div
								aria-hidden="true"
								data-sheet-thumb
								class="pointer-events-none absolute right-1 w-1 rounded-pill bg-handle"
								style:top="{thumb.top}px"
								style:height="{thumb.height}px"
							></div>
						{/if}
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
