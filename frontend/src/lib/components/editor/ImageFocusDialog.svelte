<script lang="ts" module>
	import type { ImageFocus } from '#lib/images/focus.js';

	/** One photo of the recipe, as the dialog shows and sets it. */
	export type FocusPhoto = {
		key: string;
		/** The large rendition the point is set on. */
		src: string;
		/** The photo's pixel size. */
		width: number;
		height: number;
		/** The saved point. */
		focus: ImageFocus;
		/** The disk space it takes, or a local file's size. */
		bytes: number;
		/** When it was added, as an ISO date; none while not uploaded yet. */
		added?: string;
	};
</script>

<script lang="ts">
	import { untrack } from 'svelte';
	import { Dialog } from 'bits-ui';
	import RotateCcw from '@lucide/svelte/icons/rotate-ccw';
	import Pointer from '@lucide/svelte/icons/pointer';
	import Move from '@lucide/svelte/icons/move';
	import MoveVertical from '@lucide/svelte/icons/move-vertical';
	import MoveHorizontal from '@lucide/svelte/icons/move-horizontal';
	import BookOpen from '@lucide/svelte/icons/book-open';
	import LayoutGrid from '@lucide/svelte/icons/layout-grid';
	import CalendarDays from '@lucide/svelte/icons/calendar-days';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Check from '@lucide/svelte/icons/check';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import { narrow } from '#lib/ui/narrow.js';
	import {
		alwaysVisible,
		CARD_MIN,
		cardRatio,
		CENTER,
		cropRect,
		focusPosition,
		isCentered,
		sameFocus,
		settle
	} from '#lib/images/focus.js';
	import { formatDate } from '#lib/recipe/format.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { m } from '#lib/paraglide/messages.js';

	let {
		open = $bindable(false),
		photos,
		start,
		title,
		busy = false,
		onsave
	}: {
		open?: boolean;
		/** Every photo of the recipe that has a size, in its order. */
		photos: FocusPhoto[];
		/** The key of the photo the dialog opens on. */
		start: string;
		/** The recipe's title, shown on the card and plan previews. */
		title: string;
		/** While the caller saves; the dialog stays open until it closes it. */
		busy?: boolean;
		/** Called on Save with the photos whose point changed, maybe none. */
		onsave: (changes: { key: string; focus: ImageFocus }[]) => void;
	} = $props();

	const uid = $props.id();

	// The recipe page, the card and the meal plan's square, for a photo of
	// `ratio`: the card keeps some of the photo's own shape.
	const shapesOf = (ratio: number) => [4 / 3, cardRatio(ratio), 1];
	const ratioOf = (p: FocusPhoto) => p.width / p.height;

	// The points chosen in this opening, by photo key: a photo not touched yet
	// shows its saved one. Every opening starts over; a failed save leaves the
	// dialog open, so the chosen points stay.
	let drafts = $state<Record<string, ImageFocus>>({});
	let index = $state(0);
	let touched = $state(false);
	let dragging = $state(false);
	let trackEl = $state<HTMLElement>();
	// While a finger drags the row: how far, in px.
	let offset = $state(0);
	let swiping = $state(false);
	// Only on opening, before it paints: a derived would also reset on closing
	// and move the point during the fade-out. The hint shows until the first
	// touch when the first photo has no point yet.
	$effect.pre(() => {
		if (!open) return;
		untrack(() => {
			drafts = {};
			index = Math.max(
				0,
				photos.findIndex((p) => p.key === start)
			);
			touched = !isCentered(photos[index]?.focus ?? CENTER);
			dragging = false;
			offset = 0;
		});
	});

	const pointOf = (p: FocusPhoto) =>
		drafts[p.key] ?? settle(p.focus, ratioOf(p), shapesOf(ratioOf(p)));
	const photo = $derived(photos[Math.min(index, photos.length - 1)]);
	const one = $derived(photos.length === 1);
	const ratio = $derived(ratioOf(photo));
	const card = $derived(cardRatio(ratio));
	const shapes = $derived(shapesOf(ratio));
	const point = $derived(pointOf(photo));
	// What Save sends: the photos whose point moved off the saved one.
	const changes = $derived(
		photos.flatMap((p) => {
			const draft = drafts[p.key];
			return draft && !sameFocus(draft, p.focus) ? [{ key: p.key, focus: draft }] : [];
		})
	);
	const isChanged = (p: FocusPhoto) => changes.some((c) => c.key === p.key);

	let announcement = $state('');
	function go(to: number) {
		offset = 0;
		if (to < 0 || to >= photos.length || to === index) return;
		index = to;
		shown = null;
		announcement = m.images_focus_announce({ number: to + 1, count: photos.length });
	}

	// Without zoom a photo is cut on one axis per shape: a portrait only above
	// and below, a landscape only at the sides. The point is always the middle
	// of what stays visible, so it moves only where moving changes something.
	const place = (f: ImageFocus) => (drafts[photo.key] = settle(f, ratio, shapes));
	const hole = $derived(alwaysVisible(point, ratio, shapes));
	const axisX = $derived(shapes.some((r) => r < ratio - 0.001));
	const axisY = $derived(shapes.some((r) => r > ratio + 0.001));
	const Grip = $derived(axisX && axisY ? Move : axisX ? MoveHorizontal : MoveVertical);
	const cursor = $derived(
		axisX && axisY ? 'cursor-move' : axisX ? 'cursor-ew-resize' : 'cursor-ns-resize'
	);
	// The preview under the mouse draws its own crop onto the photo.
	let shown = $state<number | null>(null);
	const shownRect = $derived(shown === null ? null : cropRect(point, ratio, shown));
	const centered = $derived(isCentered(point));
	const pos = (box: number) => focusPosition(point, ratio, box);
	const hover = (box: number) => ({
		onpointerenter: (e: PointerEvent) => {
			if (e.pointerType === 'mouse') shown = box;
		},
		onpointerleave: () => (shown = null)
	});

	// The stage: a dark band of fixed size, so nothing around it moves from
	// photo to photo. The current photo is fitted into it; its neighbors sit
	// smaller to both sides, and the row slides as one.
	let trackW = $state(0);
	let trackH = $state(0);
	const SIDE = 0.78;
	const slots = $derived.by(() => {
		const phone = narrow.current;
		const top = phone ? 22 : 28;
		const bottom = one ? top : phone ? 56 : 62;
		const boxH = Math.max(0, trackH - top - bottom);
		const maxW = one ? trackW - (phone ? 44 : 56) : trackW * (phone ? 0.64 : 0.58);
		const gap = phone ? 14 : 24;
		const size = photos.map((p) => {
			const w = Math.max(0, Math.min(maxW, boxH * ratioOf(p)));
			return { w, h: w / ratioOf(p) };
		});
		const half = (i: number) => (i === index ? size[i].w / 2 : (size[i].w * SIDE) / 2);
		const cx: number[] = [];
		cx[index] = trackW / 2 + offset;
		for (let i = index + 1; i < photos.length; i++) cx[i] = cx[i - 1] + half(i - 1) + gap + half(i);
		for (let i = index - 1; i >= 0; i--) cx[i] = cx[i + 1] - half(i + 1) - gap - half(i);
		return size.map((s, i) => ({
			...s,
			x: cx[i] - s.w / 2,
			y: top + boxH / 2 - s.h / 2,
			d: i - index
		}));
	});

	// At most seven dots; with more, the row slides along and the dots at
	// its ends shrink, so the walk through 20 photos stays one calm line.
	const DOTS = 7;
	const DOT_W = 24;
	const dotStart = $derived(
		photos.length <= DOTS ? 0 : Math.min(Math.max(index - 3, 0), photos.length - DOTS)
	);

	const clamp = (v: number) => Math.min(1, Math.max(0, v));
	function pointAt(event: PointerEvent, box: HTMLElement) {
		const r = box.getBoundingClientRect();
		place({
			x: clamp((event.clientX - r.left) / r.width),
			y: clamp((event.clientY - r.top) / r.height)
		});
	}
	function down(event: PointerEvent) {
		if (event.button !== 0) return;
		const box = event.currentTarget as HTMLElement;
		box.setPointerCapture(event.pointerId);
		touched = true;
		pointAt(event, box);
		dragging = true;
	}
	function move(event: PointerEvent) {
		if (dragging) pointAt(event, event.currentTarget as HTMLElement);
	}
	function up() {
		dragging = false;
	}

	// A sideways swipe moves the row with the finger. On the current photo it
	// only wins where the photo has no sideways axis: a portrait's point moves
	// up and down, so a sideways gesture there is clearly a swipe, and the
	// point goes back to where it was.
	let swipe: {
		id: number;
		x: number;
		y: number;
		t: number;
		mode: 'maybe' | 'swipe' | 'none';
		onCurrent: boolean;
		was: ImageFocus | undefined;
	} | null = null;
	// A click that ends a swipe is not a tap on a neighbor.
	let moved = false;
	function swipeDown(event: PointerEvent) {
		if (one || event.button !== 0) return;
		const target = event.target as Element;
		const onCurrent = !!target.closest('[data-current]');
		if (!onCurrent && target.closest('button')) return;
		swipe = {
			id: event.pointerId,
			x: event.clientX,
			y: event.clientY,
			t: performance.now(),
			mode: onCurrent && axisX ? 'none' : 'maybe',
			onCurrent,
			was: drafts[photo.key]
		};
		moved = false;
	}
	function swipeMove(event: PointerEvent) {
		if (!swipe || swipe.mode === 'none' || event.pointerId !== swipe.id) return;
		const dx = event.clientX - swipe.x;
		const dy = event.clientY - swipe.y;
		if (swipe.mode === 'maybe') {
			if (Math.abs(dx) > 10 && Math.abs(dx) > 1.4 * Math.abs(dy)) {
				swipe.mode = 'swipe';
				moved = true;
				swiping = true;
				if (swipe.onCurrent) {
					dragging = false;
					if (swipe.was) drafts[photo.key] = swipe.was;
					else delete drafts[photo.key];
				}
				trackEl?.setPointerCapture(event.pointerId);
			} else {
				if (Math.abs(dy) > 10) swipe.mode = 'none';
				return;
			}
		}
		event.stopPropagation();
		const edge = (index === 0 && dx > 0) || (index === photos.length - 1 && dx < 0);
		offset = edge ? dx * 0.35 : dx;
	}
	function swipeEnd(event: PointerEvent) {
		const s = swipe;
		swipe = null;
		if (!s || s.mode !== 'swipe') return;
		event.stopPropagation();
		swiping = false;
		const speed = offset / (performance.now() - s.t);
		if (offset < -trackW * 0.18 || speed < -0.5) go(index + 1);
		else if (offset > trackW * 0.18 || speed > 0.5) go(index - 1);
		else offset = 0;
		setTimeout(() => (moved = false));
	}
	// A trackpad's sideways scroll turns the row, one photo per gesture.
	function wheel(node: HTMLElement) {
		let sum = 0;
		let calm = 0;
		const on = (e: WheelEvent) => {
			if (one || Math.abs(e.deltaX) <= Math.abs(e.deltaY)) return;
			e.preventDefault();
			const now = performance.now();
			if (now < calm) return;
			sum += e.deltaX;
			if (Math.abs(sum) > 40) {
				go(index + (sum > 0 ? 1 : -1));
				sum = 0;
				calm = now + 420;
			}
		};
		node.addEventListener('wheel', on, { passive: false });
		return () => node.removeEventListener('wheel', on);
	}

	function key(event: KeyboardEvent) {
		if (event.target !== event.currentTarget) return;
		const step = event.shiftKey ? 0.1 : 0.02;
		const moves: Record<string, [number, number]> = {
			ArrowLeft: [-step, 0],
			ArrowRight: [step, 0],
			ArrowUp: [0, -step],
			ArrowDown: [0, step]
		};
		if (event.key in moves) {
			event.preventDefault();
			const [dx, dy] = moves[event.key];
			touched = true;
			place({ x: clamp(point.x + dx), y: clamp(point.y + dy) });
			return;
		}
		const to =
			(event.key === 'Enter' && !event.shiftKey) || event.key === 'PageDown'
				? index + 1
				: (event.key === 'Enter' && event.shiftKey) || event.key === 'PageUp'
					? index - 1
					: event.key === 'Home'
						? 0
						: event.key === 'End'
							? photos.length - 1
							: null;
		if (to === null || one) return;
		event.preventDefault();
		go(to);
	}
	const percent = $derived({ x: Math.round(point.x * 100), y: Math.round(point.y * 100) });
	const where = $derived(m.images_focus_position(percent));

	// A tap glides the point there; a drag follows the finger at once.
	const glide = $derived(
		dragging ? 'duration-0' : 'duration-200 ease-glide motion-reduce:duration-0'
	);

	// What the photo is: its shape by name, its pixels, its weight, its day.
	const RATIOS = [
		[4, 3],
		[3, 2],
		[16, 9],
		[5, 4],
		[3, 4],
		[2, 3],
		[9, 16],
		[4, 5]
	];
	const shapeName = $derived.by(() => {
		if (Math.abs(ratio - 1) < 0.02) return m.images_focus_square();
		const hit = RATIOS.find(([a, b]) => Math.abs(a / b - ratio) / ratio < 0.02);
		const named = hit && `${hit[0]}:${hit[1]}`;
		if (ratio > 1)
			return named ? m.images_focus_landscape_ratio({ ratio: named }) : m.images_focus_landscape();
		return named ? m.images_focus_portrait_ratio({ ratio: named }) : m.images_focus_portrait();
	});
	const number = (n: number) => new Intl.NumberFormat(getLocale()).format(n);
	const pixels = $derived(
		m.images_focus_pixels({ width: number(photo.width), height: number(photo.height) })
	);
	const weight = $derived(
		new Intl.NumberFormat(getLocale(), {
			style: 'unit',
			unit: 'megabyte',
			maximumFractionDigits: 1
		}).format(photo.bytes / 1e6)
	);
	const added = $derived(
		photo.added ? m.images_focus_added({ date: formatDate(photo.added) }) : m.images_focus_unsaved()
	);
	const cover = $derived(index === 0);
	// The glyph: the photo's own outline, its long side 18px.
	const glyph = $derived({
		w: (18 * Math.min(1, ratio)).toFixed(1),
		h: (18 * Math.min(1, 1 / ratio)).toFixed(1)
	});

	// While a save runs, nothing closes the dialog (Escape, a click outside, the
	// sheet's handle): a failed save must find it open with the chosen point.
	const guarded = {
		get: () => open,
		set: (v: boolean) => {
			if (v || !busy) open = v;
		}
	};

	const saveLabel = $derived(
		changes.length > 1 ? m.images_focus_save_count({ count: changes.length }) : m.common_save()
	);

	function save() {
		if (!busy) onsave(changes);
	}

	const chip =
		'inline-flex h-[30px] items-center gap-1.5 rounded-pill bg-lightbox/60 pr-3 pl-2.5 text-micro font-semibold whitespace-nowrap text-lightbox-foreground backdrop-blur-md transition';
	const navButton =
		'absolute top-1/2 z-20 grid -translate-y-1/2 place-items-center rounded-full bg-lightbox-foreground/12 text-lightbox-foreground backdrop-blur-md transition hover:bg-lightbox-foreground/25 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lightbox-primary aria-disabled:cursor-default aria-disabled:opacity-25 aria-disabled:hover:bg-lightbox-foreground/12';
	const captionClass = 'mt-1.5 flex items-center gap-1.5 text-micro font-semibold text-text-muted';
</script>

<!-- On a phone the count sits beside the title, so the help keeps the
     whole width; on a desktop it sits at the help's end, over the stage. -->
{#snippet head()}
	{@const phone = narrow.current}
	{#snippet count()}
		{#if !one}
			<span
				data-focus-count
				class="shrink-0 text-caption font-semibold whitespace-nowrap text-text-muted tabular-nums"
			>
				{m.images_focus_count({ number: index + 1, count: photos.length })}
			</span>
		{/if}
	{/snippet}
	<div class="flex justify-between gap-5 {phone ? 'items-baseline' : 'items-end'}">
		<div class="min-w-0">
			<Dialog.Title class="font-display text-heading font-medium"
				>{m.images_focus_title()}</Dialog.Title
			>
			{#if !phone}
				<Dialog.Description id="{uid}-help" class="mt-1 text-caption text-text-muted">
					{m.images_focus_help()}
				</Dialog.Description>
			{/if}
		</div>
		{@render count()}
	</div>
	{#if phone}
		<Dialog.Description id="{uid}-help" class="mt-1 text-caption text-text-muted">
			{m.images_focus_help()}
		</Dialog.Description>
	{/if}
{/snippet}

<!-- The dark stage: every photo in a row, the current one large and set by
     touch, its neighbors small beside it. -->
{#snippet stage(cls: string)}
	{@const phone = narrow.current}
	<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
	<div
		bind:this={trackEl}
		bind:clientWidth={trackW}
		bind:clientHeight={trackH}
		role="application"
		tabindex="0"
		aria-label={m.images_focus_stage()}
		aria-describedby="{uid}-help"
		class="relative overflow-clip bg-lightbox focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-lightbox-primary {cls}"
		onkeydown={key}
		onpointerdowncapture={swipeDown}
		onpointermovecapture={swipeMove}
		onpointerupcapture={swipeEnd}
		onpointercancelcapture={swipeEnd}
		{@attach wheel}
	>
		<!-- Only once the stage is measured: laid out at width 0 first, the
		     photos would glide in from its corner. -->
		{#each trackW > 0 ? photos : [] as p, i (p.key)}
			{@const s = slots[i]}
			{@const away = Math.abs(s.d)}
			{@const current = s.d === 0}
			<!-- A neighbor is a shortcut for the pointer; the keys, the arrows and
			     the dots do the same for everyone. -->
			<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
			<div
				data-slide={i}
				data-current={current ? '' : undefined}
				class="absolute top-0 left-0 origin-center {swiping
					? ''
					: 'transition-[transform,opacity,filter] duration-500 ease-glide motion-reduce:transition-none'} {current
					? ''
					: away > 2
						? 'pointer-events-none opacity-0'
						: 'cursor-pointer opacity-40 saturate-60 hover:opacity-65'}"
				style:width="{s.w}px"
				style:height="{s.h}px"
				style:z-index={10 - away}
				style:transform="translate({s.x}px, {s.y}px) scale({current ? 1 : SIDE})"
				onclick={() => {
					if (!current && !moved) go(i);
				}}
			>
				<div
					class="relative size-full touch-none overflow-hidden rounded-lg shadow-[0_18px_40px_-18px_color-mix(in_srgb,var(--color-lightbox)_90%,transparent)] select-none {current
						? cursor
						: ''}"
					onpointerdown={current ? down : undefined}
					onpointermove={current ? move : undefined}
					onpointerup={current ? up : undefined}
					onpointercancel={current ? up : undefined}
				>
					{#if away <= 3}
						<img
							src={p.src}
							alt=""
							draggable="false"
							class="pointer-events-none block size-full [-webkit-user-drag:none]"
						/>
					{/if}
					{#if current}
						<!-- What every shape shows stays clear; the rest dims. One radius
						     for the photo, the clear part and a preview's crop, so their
						     corners meet where they touch. -->
						<div
							aria-hidden="true"
							class="pointer-events-none absolute rounded-lg shadow-[0_0_0_9999px_color-mix(in_srgb,var(--color-lightbox)_52%,transparent)] outline-1 -outline-offset-1 outline-lightbox-foreground/55 transition-all {glide}"
							style:left="{hole.x * 100}%"
							style:top="{hole.y * 100}%"
							style:width="{hole.w * 100}%"
							style:height="{hole.h * 100}%"
						></div>
						{#if shownRect}
							<div
								aria-hidden="true"
								class="pointer-events-none absolute rounded-lg border-2 border-lightbox-primary transition-all duration-150 motion-reduce:transition-none"
								style:left="{shownRect.x * 100}%"
								style:top="{shownRect.y * 100}%"
								style:width="{shownRect.w * 100}%"
								style:height="{shownRect.h * 100}%"
							></div>
						{/if}
						<span
							aria-hidden="true"
							class="pointer-events-none absolute grid -translate-1/2 place-items-center rounded-full border-2 border-lightbox-foreground bg-lightbox-foreground/20 shadow-[0_2px_10px_color-mix(in_srgb,var(--color-lightbox)_45%,transparent)] backdrop-blur-[3px] transition-[left,top,scale] {glide} {phone
								? 'size-9'
								: 'size-10'} {dragging ? 'scale-115' : ''}"
							style:left="{point.x * 100}%"
							style:top="{point.y * 100}%"
						>
							<Grip class="size-4 text-lightbox-foreground" strokeWidth={2.25} />
						</span>
					{:else if isChanged(p)}
						<span
							aria-hidden="true"
							class="absolute top-2 right-2 grid size-[22px] place-items-center rounded-full bg-lightbox-primary text-lightbox"
						>
							<Check class="size-3.5" strokeWidth={3} />
						</span>
					{/if}
				</div>
			</div>
		{/each}

		{#if !touched && isCentered(photo.focus)}
			<span aria-hidden="true" class="pointer-events-none absolute top-3.5 left-3.5 z-20 {chip}">
				<Pointer class="size-3.5" aria-hidden="true" />
				{m.images_focus_hint()}
			</span>
		{/if}
		<button
			type="button"
			disabled={centered}
			onclick={() => {
				drafts[photo.key] = CENTER;
				touched = true;
				// The button hides once centered; focus goes back to the stage.
				trackEl?.focus();
			}}
			class="absolute top-3.5 right-3.5 z-20 {chip} hover:bg-lightbox/80 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lightbox-primary disabled:pointer-events-none disabled:opacity-0"
		>
			<RotateCcw class="size-3.5" aria-hidden="true" />
			{m.images_focus_center()}
		</button>

		{#if !one}
			<!-- Never disabled at the ends: a disabled button drops the focus, and
			     a walk by keyboard would lose its place. -->
			<button
				type="button"
				aria-label={m.lightbox_previous()}
				aria-disabled={index === 0}
				class="{navButton} {phone ? 'left-2 size-[38px]' : 'left-3.5 size-[46px]'}"
				onclick={() => go(index - 1)}
			>
				<ChevronLeft class="size-[22px]" aria-hidden="true" />
			</button>
			<button
				type="button"
				aria-label={m.lightbox_next()}
				aria-disabled={index === photos.length - 1}
				class="{navButton} {phone ? 'right-2 size-[38px]' : 'right-3.5 size-[46px]'}"
				onclick={() => go(index + 1)}
			>
				<ChevronRight class="size-[22px]" aria-hidden="true" />
			</button>

			<div class="pointer-events-none absolute inset-x-0 bottom-4 z-20 flex justify-center">
				<div
					role="group"
					aria-label={m.images_thumbnails()}
					class="pointer-events-auto overflow-hidden"
					style:width="{Math.min(photos.length, DOTS) * DOT_W}px"
				>
					<div
						class="flex transition-transform duration-500 ease-glide motion-reduce:transition-none"
						style:transform="translateX({-dotStart * DOT_W}px)"
					>
						{#each photos as p, i (p.key)}
							{@const out = i < dotStart || i >= dotStart + DOTS}
							{@const edge =
								(i === dotStart && dotStart > 0) ||
								(i === dotStart + DOTS - 1 && dotStart + DOTS < photos.length)}
							{@const done = isChanged(p)}
							<button
								type="button"
								tabindex={out ? -1 : 0}
								aria-label={done
									? m.images_focus_photo_changed({ number: i + 1 })
									: m.images_show({ number: i + 1 })}
								aria-current={i === index ? 'true' : undefined}
								class="group/dot grid h-[30px] shrink-0 place-items-center focus-visible:outline-none"
								style:width="{DOT_W}px"
								onclick={() => go(i)}
							>
								<i
									class="block h-2 rounded-full transition-[width,background-color,transform,opacity] duration-500 ease-glide group-focus-visible/dot:outline-2 group-focus-visible/dot:outline-offset-2 group-focus-visible/dot:outline-lightbox-primary motion-reduce:transition-none {i ===
									index
										? 'w-[18px]'
										: 'w-2'} {done
										? 'bg-lightbox-primary'
										: i === index
											? 'bg-lightbox-foreground'
											: 'bg-lightbox-foreground/35'} {done && i === index
										? 'ring-2 ring-lightbox-foreground'
										: ''} {out ? 'scale-30 opacity-0' : edge ? 'scale-50' : ''}"
								></i>
							</button>
						{/each}
					</div>
				</div>
			</div>
		{/if}
	</div>
	<p data-focus-position class="sr-only" aria-live="polite">{where}</p>
	<p class="sr-only" aria-live="polite">{announcement}</p>
{/snippet}

<!-- Each preview is the place itself in small: the recipe page's cover, a
     recipe card with its title, a meal plan entry. Pointing at one draws its
     crop onto the photo. -->
{#snippet previews(phone: boolean)}
	{@const long = !phone}
	<figure class="m-0" {...hover(4 / 3)}>
		<img
			src={photo.src}
			alt=""
			class="block aspect-[4/3] w-full rounded-xl object-cover shadow-cover transition-[object-position] duration-200 motion-reduce:transition-none"
			style:object-position={pos(4 / 3)}
		/>
		<figcaption class={captionClass}>
			<BookOpen class="size-3.5 shrink-0" aria-hidden="true" />
			{long ? m.images_focus_page() : m.images_focus_page_short()}
		</figcaption>
	</figure>
	<figure class="m-0 {long ? 'w-[112px]' : ''}" {...hover(card)}>
		<!-- The card's slot is as tall as its tallest shape and the card sits
		     on its floor: from photo to photo the previews never move. -->
		<div class="grid">
			{#snippet face(ghost: boolean)}
				<div
					aria-hidden={ghost ? 'true' : undefined}
					class="col-start-1 row-start-1 rounded-xl bg-surface p-1 shadow-card {ghost
						? 'invisible'
						: 'self-end'}"
				>
					{#if ghost}
						<div style:aspect-ratio={CARD_MIN}></div>
					{:else}
						<img
							src={photo.src}
							alt=""
							class="block w-full rounded-lg object-cover transition-[object-position] duration-200 motion-reduce:transition-none"
							style:aspect-ratio={card}
							style:object-position={pos(card)}
						/>
					{/if}
					{#if long}
						<p
							class="line-clamp-2 px-1 pt-1.5 pb-1 font-display text-micro leading-tight font-medium text-text"
						>
							{title}
						</p>
					{/if}
				</div>
			{/snippet}
			{@render face(true)}
			{@render face(false)}
		</div>
		<figcaption class={captionClass}>
			<LayoutGrid class="size-3.5 shrink-0" aria-hidden="true" />
			{long ? m.images_focus_card() : m.images_focus_card_short()}
		</figcaption>
	</figure>
	<figure class="m-0" {...hover(1)}>
		<div class="flex items-center gap-2 rounded-xl bg-surface p-1 shadow-card {long ? 'pr-2' : ''}">
			<img
				src={photo.src}
				alt=""
				class="block aspect-square shrink-0 rounded-lg object-cover transition-[object-position] duration-200 motion-reduce:transition-none {long
					? 'size-10'
					: 'w-full'}"
				style:object-position={pos(1)}
			/>
			{#if long}
				<span class="line-clamp-2 min-w-0 text-micro leading-tight font-semibold text-text"
					>{title}</span
				>
			{/if}
		</div>
		<figcaption class={captionClass}>
			<CalendarDays class="size-3.5 shrink-0" aria-hidden="true" />
			{long ? m.images_focus_plan() : m.images_focus_plan_short()}
		</figcaption>
	</figure>
{/snippet}

{#if narrow.current}
	<!-- As tall as a sheet goes: the stage takes what the rest leaves. -->
	<BottomSheet bind:open={guarded.get, guarded.set} closeLabel={m.common_close()} fill>
		<div data-sheet-body class="flex min-h-0 flex-1 flex-col overflow-y-auto px-5 pb-4">
			{@render head()}
			<div class="mt-3.5 flex max-h-[380px] min-h-[190px] flex-1 basis-0">
				{@render stage('size-full rounded-3xl')}
			</div>
			<p
				data-focus-facts
				class="mt-2 shrink-0 truncate text-center text-micro font-semibold text-text-muted tabular-nums"
			>
				{#if cover}<b class="text-text">{m.images_cover_badge()}</b> ·
				{/if}{shapeName} · {pixels} · {weight}
			</p>
			<div class="mt-2.5 grid shrink-0 grid-cols-[4fr_3fr_3fr] items-end gap-3">
				{@render previews(true)}
			</div>
		</div>
		<!-- Cancel takes what its word needs and Save the rest: "Save 3 photos"
		     stays on one line at 320px. -->
		<div class="flex gap-3 border-t border-border px-5 pt-3.5 pb-5">
			<Button variant="ghost" class="shrink-0" disabled={busy} onclick={() => (open = false)}
				>{m.common_cancel()}</Button
			>
			<Button class="min-w-0 flex-1 whitespace-nowrap" {busy} onclick={save}>{saveLabel}</Button>
		</div>
	</BottomSheet>
{:else}
	<!-- The stage takes the focus, so the arrow keys move the point at once. -->
	<BaseDialog
		bind:open={guarded.get, guarded.set}
		wide="stage"
		onOpenAutoFocus={(e) => {
			e.preventDefault();
			trackEl?.focus();
		}}
	>
		{@render head()}
		<!-- One frame, two halves: the photos on their dark stage, and where the
		     current one shows in the app beside it. -->
		<div
			class="mt-5 grid h-[500px] grid-cols-[minmax(0,1fr)_212px] grid-rows-[minmax(0,1fr)] overflow-hidden rounded-2xl ring-1 ring-border"
		>
			{@render stage('h-full')}
			<div class="flex min-h-0 flex-col justify-between overflow-hidden bg-background p-4">
				{@render previews(false)}
			</div>
		</div>
		<div class="mt-5 flex items-center gap-4 border-t border-border pt-4">
			<div data-focus-facts class="flex min-w-0 items-center gap-3">
				<span
					aria-hidden="true"
					class="grid size-[34px] shrink-0 place-items-center rounded-md bg-background"
				>
					<i
						class="block rounded-word border-[1.5px] border-text-muted bg-text-muted/20 transition-[width,height] duration-300 ease-glide motion-reduce:transition-none"
						style:width="{glyph.w}px"
						style:height="{glyph.h}px"
					></i>
				</span>
				<span class="min-w-0">
					<span class="block truncate text-caption font-semibold text-text">
						{shapeName}
						{#if cover}
							<span
								class="ml-1.5 inline-block rounded-pill bg-accent px-2 text-micro text-accent-foreground"
								>{m.images_cover_badge()}</span
							>
						{/if}
					</span>
					<span class="block truncate text-micro font-semibold text-text-muted tabular-nums">
						{pixels} · {weight} · {added}
					</span>
				</span>
			</div>
			{#if !one}
				<span
					class="ml-auto flex shrink-0 items-center gap-1.5 text-micro font-semibold whitespace-nowrap text-text-muted"
				>
					<kbd
						class="inline-grid h-5 min-w-5 place-items-center rounded-word border border-border px-1 font-sans text-micro"
						>Enter</kbd
					>
					{m.images_focus_next_key()}
				</span>
			{/if}
			<DialogActions class="shrink-0 {one ? 'ml-auto' : ''}">
				<Button variant="ghost" disabled={busy} onclick={() => (open = false)}
					>{m.common_cancel()}</Button
				>
				<Button {busy} onclick={save}>{saveLabel}</Button>
			</DialogActions>
		</div>
	</BaseDialog>
{/if}
