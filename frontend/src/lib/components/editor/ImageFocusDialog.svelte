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
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import { narrow } from '#lib/ui/narrow.js';
	import {
		alwaysVisible,
		cardRatio,
		CENTER,
		cropRect,
		focusPosition,
		settle,
		type ImageFocus
	} from '#lib/images/focus.js';
	import { m } from '#lib/paraglide/messages.js';

	let {
		open = $bindable(false),
		src,
		width,
		height,
		title,
		focus,
		busy = false,
		onsave
	}: {
		open?: boolean;
		src: string;
		/** The photo's pixel size; only its ratio matters. */
		width: number;
		height: number;
		/** The recipe's title, shown on the card and plan previews. */
		title: string;
		/** The saved point the dialog starts from. */
		focus: ImageFocus;
		/** While the caller saves; the dialog stays open until it closes it. */
		busy?: boolean;
		/** Called on Save, also with an unchanged point; the caller skips the request. */
		onsave: (focus: ImageFocus) => void;
	} = $props();

	const uid = $props.id();

	const ratio = $derived(width / height);
	const card = $derived(cardRatio(ratio));
	// The recipe page, the card and the meal plan's square.
	const shapes = $derived([4 / 3, card, 1]);

	const isCentered = (f: ImageFocus) => Math.abs(f.x - 0.5) < 0.005 && Math.abs(f.y - 0.5) < 0.005;

	// The point being chosen. Every opening starts over from the saved one; a
	// failed save leaves the dialog open, so the chosen point stays. The hint
	// shows until the first touch on a photo that has no point yet.
	let point = $state<ImageFocus>(CENTER);
	let touched = $state(false);
	let dragging = $state(false);
	let stageEl = $state<HTMLElement>();
	// Only on opening, before it paints: a derived would also reset on closing
	// and move the point during the fade-out.
	$effect.pre(() => {
		if (!open) return;
		untrack(() => {
			point = settle(focus, ratio, shapes);
			touched = !isCentered(point);
			dragging = false;
		});
	});

	// Without zoom a photo is cut on one axis per shape: a portrait only above
	// and below, a landscape only at the sides. The point is always the middle
	// of what stays visible, so it moves only where moving changes something.
	const place = (f: ImageFocus) => (point = settle(f, ratio, shapes));
	const hole = $derived(alwaysVisible(point, ratio, shapes));
	const axisX = $derived(shapes.some((r) => r < ratio - 0.001));
	const axisY = $derived(shapes.some((r) => r > ratio + 0.001));
	const Grip = $derived(axisX && axisY ? Move : axisX ? MoveHorizontal : MoveVertical);
	const cursor = $derived(
		axisX && axisY ? 'cursor-move' : axisX ? 'cursor-ew-resize' : 'cursor-ns-resize'
	);
	// The preview under the pointer draws its own crop onto the photo.
	let shown = $state<number | null>(null);
	const shownRect = $derived(shown === null ? null : cropRect(point, ratio, shown));
	const centered = $derived(isCentered(point));
	const pos = (box: number) => focusPosition(point, ratio, box);
	const captionClass = 'mt-2 flex items-center gap-1.5 text-micro font-semibold text-text-muted';

	// The stage takes the photo's shape, within a band: a landscape photo
	// leaves no black bars above and below, a tall one no endless column.
	let stageW = $state(0);
	let innerH = $state(800);
	const PAD = 24;
	const stageH = $derived(
		narrow.current
			? Math.min(Math.round(innerH * 0.33), Math.max(200, (stageW - PAD) / ratio + PAD))
			: Math.min(452, Math.max(380, (stageW - PAD) / ratio + PAD))
	);
	// The photo fitted into the stage.
	let boxW = $state(0);
	let boxH = $state(0);
	const fitW = $derived(Math.min(boxW, boxH * ratio));
	const fitH = $derived(fitW / ratio);

	const clamp = (v: number) => Math.min(1, Math.max(0, v));
	function pointAt(event: PointerEvent) {
		const r = (event.currentTarget as HTMLElement).getBoundingClientRect();
		place({
			x: clamp((event.clientX - r.left) / r.width),
			y: clamp((event.clientY - r.top) / r.height)
		});
	}
	function down(event: PointerEvent) {
		if (event.button !== 0) return;
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
		touched = true;
		pointAt(event);
		dragging = true;
	}
	function move(event: PointerEvent) {
		if (dragging) pointAt(event);
	}
	function up() {
		dragging = false;
	}
	function key(event: KeyboardEvent) {
		const step = event.shiftKey ? 0.1 : 0.02;
		const moves: Record<string, [number, number]> = {
			ArrowLeft: [-step, 0],
			ArrowRight: [step, 0],
			ArrowUp: [0, -step],
			ArrowDown: [0, step]
		};
		if (!(event.key in moves)) return;
		event.preventDefault();
		const [dx, dy] = moves[event.key];
		touched = true;
		place({ x: clamp(point.x + dx), y: clamp(point.y + dy) });
	}
	const percent = $derived({ x: Math.round(point.x * 100), y: Math.round(point.y * 100) });
	const where = $derived(m.images_focus_position(percent));

	// A tap glides the point there; a drag follows the finger at once.
	const glide = $derived(
		dragging ? 'duration-0' : 'duration-200 ease-out motion-reduce:duration-0'
	);

	// While a save runs, nothing closes the dialog (Escape, a click outside, the
	// sheet's handle): a failed save must find it open with the chosen point.
	const guarded = {
		get: () => open,
		set: (v: boolean) => {
			if (v || !busy) open = v;
		}
	};

	function save() {
		if (!busy) onsave(point);
	}
</script>

<svelte:window bind:innerHeight={innerH} />

{#snippet head()}
	<Dialog.Title class="font-display text-heading font-medium">{m.images_focus_title()}</Dialog.Title
	>
	<Dialog.Description id="{uid}-help" class="mt-1 text-caption text-text-muted">
		{m.images_focus_help()}
	</Dialog.Description>
{/snippet}

{#snippet stage(cls: string)}
	<div
		bind:clientWidth={stageW}
		class="relative bg-lightbox {cls}"
		style:height={narrow.current ? `${stageH}px` : undefined}
		style:min-height={narrow.current ? undefined : `${stageH}px`}
	>
		<div
			bind:clientWidth={boxW}
			bind:clientHeight={boxH}
			class="absolute inset-3 grid place-items-center"
		>
			<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
			<div
				bind:this={stageEl}
				role="application"
				tabindex="0"
				aria-label={m.images_focus_point(percent)}
				aria-describedby="{uid}-help"
				class="relative {cursor} touch-none overflow-hidden rounded-lg outline-none select-none focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-lightbox-primary"
				style:width="{fitW}px"
				style:height="{fitH}px"
				onpointerdown={down}
				onpointermove={move}
				onpointerup={up}
				onpointercancel={up}
				onkeydown={key}
			>
				<img {src} alt="" draggable="false" class="block size-full [-webkit-user-drag:none]" />
				<!-- What every shape shows stays clear; the rest dims. -->
				<div
					aria-hidden="true"
					class="pointer-events-none absolute rounded-md shadow-[0_0_0_9999px_color-mix(in_srgb,var(--color-lightbox)_52%,transparent)] outline-1 outline-lightbox-foreground/55 transition-all {glide}"
					style:left="{hole.x * 100}%"
					style:top="{hole.y * 100}%"
					style:width="{hole.w * 100}%"
					style:height="{hole.h * 100}%"
				></div>
				{#if shownRect}
					<div
						aria-hidden="true"
						class="pointer-events-none absolute rounded-md border-2 border-lightbox-primary transition-all duration-150 motion-reduce:transition-none"
						style:left="{shownRect.x * 100}%"
						style:top="{shownRect.y * 100}%"
						style:width="{shownRect.w * 100}%"
						style:height="{shownRect.h * 100}%"
					></div>
				{/if}
				<span
					aria-hidden="true"
					class="pointer-events-none absolute grid size-10 -translate-1/2 place-items-center rounded-full border-2 border-lightbox-foreground bg-lightbox-foreground/20 shadow-[0_2px_10px_color-mix(in_srgb,var(--color-lightbox)_45%,transparent)] backdrop-blur-[3px] transition-[left,top,scale] {glide} {dragging
						? 'scale-115'
						: ''}"
					style:left="{point.x * 100}%"
					style:top="{point.y * 100}%"
				>
					<Grip class="size-4 text-lightbox-foreground" strokeWidth={2.25} />
				</span>
			</div>
		</div>
		{#if !touched}
			<span
				aria-hidden="true"
				class="pointer-events-none absolute bottom-3 left-1/2 inline-flex -translate-x-1/2 items-center gap-1.5 rounded-pill bg-lightbox/65 px-3 py-1.5 text-micro font-semibold whitespace-nowrap text-lightbox-foreground backdrop-blur-md"
			>
				<Pointer class="size-3.5" aria-hidden="true" />
				{m.images_focus_hint()}
			</span>
		{/if}
		<button
			type="button"
			disabled={centered}
			onclick={() => {
				point = CENTER;
				// The button hides once centered; focus goes back to the photo.
				stageEl?.focus();
			}}
			class="absolute top-3 right-3 inline-flex items-center gap-1.5 rounded-pill bg-lightbox/65 py-1.5 pr-3 pl-2.5 text-micro font-semibold text-lightbox-foreground backdrop-blur-md transition hover:bg-lightbox/80 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lightbox-primary disabled:pointer-events-none disabled:opacity-0"
		>
			<RotateCcw class="size-3.5" aria-hidden="true" />
			{m.images_focus_center()}
		</button>
	</div>
	<p class="sr-only" aria-live="polite">{where}</p>
{/snippet}

<!-- Each preview is the place itself in small: the recipe page's cover, a
     recipe card with its title, a meal plan entry. Pointing at one draws its
     crop onto the photo. -->
{#snippet caption(Icon: typeof BookOpen, long: string, short: string)}
	<Icon class="size-3.5 shrink-0" aria-hidden="true" />
	<span class="max-md:hidden">{long}</span><span class="md:hidden">{short}</span>
{/snippet}

{#snippet pagePreview(cls: string)}
	<figure class={cls} onpointerenter={() => (shown = 4 / 3)} onpointerleave={() => (shown = null)}>
		<img
			{src}
			alt=""
			class="block aspect-[4/3] w-full rounded-xl object-cover shadow-cover"
			style:object-position={pos(4 / 3)}
		/>
		<figcaption class={captionClass}>
			{@render caption(BookOpen, m.images_focus_page(), m.images_focus_page_short())}
		</figcaption>
	</figure>
{/snippet}

{#snippet cardPreview(cls: string)}
	<figure class={cls} onpointerenter={() => (shown = card)} onpointerleave={() => (shown = null)}>
		<div class="rounded-xl bg-surface p-1 shadow-card">
			<img
				{src}
				alt=""
				class="block w-full rounded-lg object-cover"
				style:aspect-ratio={card}
				style:object-position={pos(card)}
			/>
			<p
				class="line-clamp-2 px-1 pt-1.5 pb-1 font-display text-micro leading-tight font-medium text-text max-md:hidden"
			>
				{title}
			</p>
		</div>
		<figcaption class={captionClass}>
			{@render caption(LayoutGrid, m.images_focus_card(), m.images_focus_card_short())}
		</figcaption>
	</figure>
{/snippet}

{#snippet planPreview(cls: string)}
	<figure class={cls} onpointerenter={() => (shown = 1)} onpointerleave={() => (shown = null)}>
		<div class="flex items-center gap-2 rounded-xl bg-surface p-1 shadow-card md:pr-2">
			<img
				{src}
				alt=""
				class="block aspect-square shrink-0 rounded-lg object-cover max-md:w-full md:size-10"
				style:object-position={pos(1)}
			/>
			<span
				class="line-clamp-2 min-w-0 text-micro leading-tight font-semibold text-text max-md:hidden"
				>{title}</span
			>
		</div>
		<figcaption class={captionClass}>
			{@render caption(CalendarDays, m.images_focus_plan(), m.images_focus_plan_short())}
		</figcaption>
	</figure>
{/snippet}

{#if narrow.current}
	<!-- The sheet focuses itself on opening; its handle would wear a square ring. -->
	<BottomSheet bind:open={guarded.get, guarded.set} closeLabel={m.common_close()}>
		<div data-sheet-body class="flex min-h-0 flex-1 flex-col overflow-y-auto px-5 pb-4">
			{@render head()}
			{@render stage('mt-4 shrink-0 rounded-2xl ring-1 ring-border')}
			<div class="mt-4 grid grid-cols-[4fr_3fr_3fr] items-end gap-3">
				{@render pagePreview('')}
				{@render cardPreview('')}
				{@render planPreview('')}
			</div>
		</div>
		<div class="flex gap-3 border-t border-border px-5 pt-3.5 pb-5">
			<Button variant="ghost" class="flex-1" disabled={busy} onclick={() => (open = false)}
				>{m.common_cancel()}</Button
			>
			<Button class="flex-1" {busy} onclick={save}>{m.common_save()}</Button>
		</div>
	</BottomSheet>
{:else}
	<BaseDialog bind:open={guarded.get, guarded.set} wide onOpenAutoFocus={(e) => e.preventDefault()}>
		{@render head()}
		<!-- One frame, two halves: the photo to set on its dark stage, and
		     where it shows in the app beside it. -->
		<div
			class="mt-5 grid grid-cols-[minmax(0,1fr)_212px] overflow-hidden rounded-2xl ring-1 ring-border"
		>
			{@render stage('')}
			<div class="flex flex-col justify-between gap-4 bg-background p-4">
				{@render pagePreview('')}
				{@render cardPreview('w-[128px]')}
				{@render planPreview('')}
			</div>
		</div>
		<DialogActions class="mt-6 border-t border-border pt-4">
			<Button variant="ghost" disabled={busy} onclick={() => (open = false)}
				>{m.common_cancel()}</Button
			>
			<Button {busy} onclick={save}>{m.common_save()}</Button>
		</DialogActions>
	</BaseDialog>
{/if}
