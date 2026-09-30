<script lang="ts">
	import { Dialog, Slider } from 'bits-ui';
	import { SvelteMap } from 'svelte/reactivity';
	import BaseDialog from '$lib/components/ui/BaseDialog.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';
	import {
		centered,
		clampView,
		MAX_ZOOM,
		MIN_ZOOM,
		scaleOf,
		toCrop,
		zoomAt,
		type Crop,
		type View
	} from '$lib/user/crop';

	let {
		open = $bindable(false),
		file,
		busy = false,
		onconfirm
	}: {
		open?: boolean;
		/** The picked file; the dialog shows it and reports a crop of it. */
		file: File | null;
		busy?: boolean;
		onconfirm: (crop: Crop) => void;
	} = $props();

	const uid = $props.id();

	let frame = $state(0);
	let natural = $state<{ w: number; h: number } | null>(null);
	let view = $state<View>({ zoom: 1, x: 0, y: 0 });

	// An object URL per file, revoked when the file changes or the dialog goes.
	const src = $derived(file ? URL.createObjectURL(file) : null);
	$effect(() => {
		const url = src;
		return () => {
			if (url) URL.revokeObjectURL(url);
		};
	});

	function loaded(event: Event) {
		const img = event.currentTarget as HTMLImageElement;
		natural = { w: img.naturalWidth, h: img.naturalHeight };
		view = centered(natural.w, natural.h, frame);
	}

	const scale = $derived(natural ? scaleOf(natural.w, natural.h, frame, view.zoom) : 1);

	function set(next: View) {
		if (natural) view = clampView(natural.w, natural.h, frame, next);
	}
	function zoomTo(zoom: number, px = frame / 2, py = frame / 2) {
		if (natural) view = zoomAt(natural.w, natural.h, frame, view, zoom, px, py);
	}

	// Pointers down on the frame, for a drag with one and a pinch with two.
	const pointers = new SvelteMap<number, { x: number; y: number }>();
	let pinch = 0;

	function down(event: PointerEvent) {
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
		pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
		if (pointers.size === 2) pinch = distance();
	}
	function move(event: PointerEvent) {
		const last = pointers.get(event.pointerId);
		if (!last) return;
		const next = { x: event.clientX, y: event.clientY };
		pointers.set(event.pointerId, next);
		if (pointers.size === 1) {
			set({ ...view, x: view.x + next.x - last.x, y: view.y + next.y - last.y });
		} else if (pointers.size === 2 && pinch > 0) {
			const d = distance();
			const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
			const [a, b] = [...pointers.values()];
			zoomTo(view.zoom * (d / pinch), (a.x + b.x) / 2 - rect.left, (a.y + b.y) / 2 - rect.top);
			pinch = d;
		}
	}
	function up(event: PointerEvent) {
		pointers.delete(event.pointerId);
		pinch = 0;
	}
	function distance() {
		const [a, b] = [...pointers.values()];
		return Math.hypot(a.x - b.x, a.y - b.y);
	}
	function wheel(event: WheelEvent) {
		event.preventDefault();
		const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
		zoomTo(
			view.zoom * Math.exp(-event.deltaY * 0.002),
			event.clientX - rect.left,
			event.clientY - rect.top
		);
	}
	function key(event: KeyboardEvent) {
		const step = 10;
		const moves: Record<string, [number, number]> = {
			ArrowLeft: [step, 0],
			ArrowRight: [-step, 0],
			ArrowUp: [0, step],
			ArrowDown: [0, -step]
		};
		if (event.key in moves) {
			const [dx, dy] = moves[event.key];
			set({ ...view, x: view.x + dx, y: view.y + dy });
		} else if (event.key === '+' || event.key === '=') {
			zoomTo(view.zoom + 0.1);
		} else if (event.key === '-') {
			zoomTo(view.zoom - 0.1);
		} else {
			return;
		}
		event.preventDefault();
	}

	function confirm() {
		if (natural) onconfirm(toCrop(natural.w, natural.h, frame, view));
	}
</script>

<BaseDialog bind:open>
	<Dialog.Title class="font-display text-heading font-medium">{m.avatar_crop_title()}</Dialog.Title>
	<Dialog.Description id="{uid}-help" class="mt-1 text-caption text-text-muted">
		{m.avatar_crop_description()}
	</Dialog.Description>
	<!--
		The frame is a square the image covers; a round window inside it shows
		what the circles will show. The dimming outside the window is the
		overlay token drawn as an outsized shadow, so the corners stay dim
		however the image moves underneath.
	-->
	<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
	<div
		bind:clientWidth={frame}
		role="application"
		tabindex="0"
		aria-label={m.avatar_crop_frame()}
		aria-describedby="{uid}-help"
		class="relative mt-5 aspect-square w-full touch-none overflow-hidden rounded-2xl bg-background select-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
		onpointerdown={down}
		onpointermove={move}
		onpointerup={up}
		onpointercancel={up}
		onwheel={wheel}
		onkeydown={key}
	>
		{#if src}
			<img
				{src}
				alt=""
				draggable="false"
				onload={loaded}
				class="absolute top-0 left-0 max-w-none origin-top-left"
				style:width={natural ? `${natural.w * scale}px` : undefined}
				style:transform="translate({view.x}px, {view.y}px)"
			/>
		{/if}
		<div
			aria-hidden="true"
			class="pointer-events-none absolute inset-0 rounded-full shadow-[0_0_0_9999px_var(--color-overlay)]"
		></div>
	</div>
	<Slider.Root
		type="single"
		min={MIN_ZOOM}
		max={MAX_ZOOM}
		step={0.01}
		value={view.zoom}
		onValueChange={(zoom) => zoomTo(zoom)}
		class="relative mt-4 flex w-full touch-none items-center py-2.5 select-none"
	>
		<span class="relative h-1.5 w-full grow rounded-pill bg-background">
			<Slider.Range class="absolute h-full rounded-pill bg-primary" />
			<Slider.Thumb
				index={0}
				aria-label={m.avatar_crop_zoom()}
				class="top-1/2 -mt-3 block size-6 rounded-full border-[1.5px] border-primary bg-surface-elevated shadow-card transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			/>
		</span>
	</Slider.Root>
	<div class="flex justify-end gap-3 pt-5">
		<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
		<Button onclick={confirm} disabled={busy || !natural}>{m.common_save()}</Button>
	</div>
</BaseDialog>
