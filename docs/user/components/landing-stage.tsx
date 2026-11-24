'use client';
// The landing page's hero: one recipe on a laptop and a phone, with a light/dark split the visitor drags.
import { ChevronsLeftRight, Moon, Sun } from 'lucide-react';
import { useEffect, useRef, type CSSProperties, type KeyboardEvent, type PointerEvent } from 'react';

type Scheme = 'light' | 'dark';

const src = (name: string, device: 'desktop' | 'mobile', scheme: Scheme) => `/screenshots/${name}-${device}-${scheme}.png`;

/** The screens on stage. screenshots-audit.ts reads each shot name, so a renamed test fails the audit, not the picture. */
const SHOTS = {
	laptop: { shot: 'recipe-detail', device: 'desktop' },
	phone: { shot: 'cook-mode-timers', device: 'mobile' }
} as const;

/** The viewports the screenshot projects shoot at (playwright.config.ts); the frames take the shots' aspect from them. */
const SIZE = { desktop: [1440, 900], mobile: [360, 780] } as const;

/** A 1px transparent GIF: what a `<picture>` shows below or above its breakpoint, so an eager shot only loads where it is seen. */
const BLANK = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7';

/**
 * One screen: the light and the dark shot of `name` stacked in the same box, so the frame around them is one element and
 * only the picture changes at the divider. The stage clips the light shot (`data-light`) to the left of the split and the dark
 * one (`data-dark`) to the right, so the two never overlap and no antialiased light edge shows through on the dark side.
 * Each shot loads only on its side of md: Chrome fetches even lazy images inside a display:none scene once they are near
 * the viewport, so the other side gets a blank source. `priority` marks the shot that paints the hero first.
 */
function Screen({ screen, alt, only, priority, className }: { screen: (typeof SHOTS)[keyof typeof SHOTS]; alt: string; only: 'md' | 'phone'; priority?: boolean; className: string }) {
	const { shot: name, device } = screen;
	const [w, h] = SIZE[device];
	const media = only === 'md' ? '(max-width: 767.98px)' : '(min-width: 768px)';
	return (
		<div className="relative">
			<picture>
				<source media={media} srcSet={BLANK} />
				<img data-light src={src(name, device, 'light')} alt={alt} width={w} height={h} decoding="async" fetchPriority={priority ? 'high' : undefined} className={`block w-full ${className}`} />
			</picture>
			<picture>
				<source media={media} srcSet={BLANK} />
				{/* Hidden until the stage measures the split: without JavaScript the screens simply show light. */}
				<img data-dark src={src(name, device, 'dark')} alt="" width={w} height={h} decoding="async" loading="lazy" style={{ clipPath: 'inset(0 0 0 100%)' }} className={`absolute inset-0 size-full ${className}`} />
			</picture>
		</div>
	);
}

/*
 * Laptop and phone (from md up) or the phone alone (below md), rendered once. The hardware is always dark, like the real thing.
 * The entrance is a CSS transition from @starting-style, so without JavaScript the devices simply appear.
 */
function Scene() {
	const enter = 'transition-[opacity,translate,rotate] duration-1000 ease-out motion-reduce:transition-none';
	return (
		<>
			{/* md and up: the laptop with the phone in front of its right corner. */}
			<div className="relative pb-[7%] max-md:hidden">
				<div className={`w-[82%] ${enter} starting:translate-y-10 starting:opacity-0`}>
					{/* On a dark page a faint light ring keeps the dark hardware from dissolving into the page. */}
					<div className="dark rounded-t-2xl border border-b-0 border-fd-border bg-fd-background p-[1.1%] pb-[1.4%] shadow-2xl [:root.dark_&]:outline [:root.dark_&]:outline-1 [:root.dark_&]:outline-fd-card-foreground/12">
						<div className="mx-auto mb-[0.7%] size-1.5 rounded-full bg-fd-border" />
						<Screen screen={SHOTS.laptop} alt="Shepherd’s Pie on the laptop, and in cook mode on the phone with timers running" only="md" priority className="rounded-sm" />
					</div>
					{/* The base: a little wider than the screen, with the thumb notch. */}
					<div className="dark relative -mx-[4%] h-3.5 rounded-b-2xl border border-fd-border bg-fd-muted shadow-lg lg:h-4">
						<div className="absolute top-0 left-1/2 h-1.5 w-[14%] -translate-x-1/2 rounded-b-md bg-fd-background" />
					</div>
				</div>
				<div data-tilt className={`absolute right-[4%] bottom-0 w-[23%] origin-bottom rotate-[-3deg] delay-200 ${enter} starting:translate-x-16 starting:translate-y-6 starting:rotate-[4deg] starting:opacity-0`}>
					<Phone alt="" only="md" />
				</div>
			</div>
			{/* Below md: the phone alone, large, cut off and faded at the bottom, so the page does not open on a whole screenshot. */}
			<div className={`mx-auto max-h-[58svh] w-[min(78%,300px)] overflow-hidden mask-b-from-70% md:hidden ${enter} starting:translate-y-8 starting:opacity-0`}>
				<Phone alt="Shepherd’s Pie in cook mode on a phone, with timers running" only="phone" priority />
			</div>
		</>
	);
}

function Phone({ alt, only, priority }: { alt: string; only: 'md' | 'phone'; priority?: boolean }) {
	return (
		<div className="rounded-4xl shadow-2xl dark:shadow-md dark:ring-1 dark:ring-fd-card-foreground/12">
			<div className="dark rounded-4xl border border-fd-border bg-fd-background p-[3.5%] ring-1 ring-fd-muted">
				<Screen screen={SHOTS.phone} alt={alt} only={only} priority={priority} className="rounded-3xl" />
			</div>
		</div>
	);
}

/**
 * Clips every dark shot in `stage` to the right of the screen-space line x = `x`, every light shot to its left. A tilted shot (the phone) gets a tilted
 * polygon in its own coordinates, so the cut stays exactly vertical on screen: a rotated box's bounding rect is centered on
 * the box, and offsetWidth/Height are its untransformed size.
 */
function clipScreens(stage: HTMLElement, x: number) {
	for (const img of stage.querySelectorAll<HTMLImageElement>('img[data-light], img[data-dark]')) {
		const { offsetWidth: w, offsetHeight: h } = img;
		if (!w) continue; // the scene on the other side of md
		const r = img.getBoundingClientRect();
		// Tailwind's rotate-* sets the `rotate` property (mid-entrance too), not `transform`.
		const tilt = img.closest<HTMLElement>('[data-tilt]');
		const a = ((tilt ? parseFloat(getComputedStyle(tilt).rotate) || 0 : 0) * Math.PI) / 180;
		const [cos, sin] = [Math.cos(a), Math.sin(a)];
		const cx = r.left + r.width / 2;
		const u = (v: number) => w / 2 + (x - cx + sin * (v - h / 2)) / cos;
		const [top, bottom] = [u(0), u(h)];
		img.style.clipPath = img.hasAttribute('data-dark')
			? `polygon(${top}px 0, ${w + 2}px 0, ${w + 2}px ${h}px, ${bottom}px ${h}px)`
			: `polygon(-2px 0, ${top}px 0, ${bottom}px ${h}px, -2px ${h}px)`;
	}
}

/** Where the split rests, and how far and how long its one hint sweep goes. */
const REST = 60;
const SWEEP = 30;
const SWEEP_MS = 4500;

export function LandingStage() {
	const stage = useRef<HTMLDivElement>(null);
	const handle = useRef<HTMLDivElement>(null);
	const touched = useRef(false);
	const dragging = useRef(false);

	const split = useRef(REST);

	// Write the split straight to the DOM: it changes every frame during the sweep, a re-render per frame buys nothing.
	const apply = (pct: number) => {
		const p = Math.min(100, Math.max(0, pct));
		split.current = p;
		const el = stage.current;
		if (!el) return;
		el.style.setProperty('--split', `${p}%`);
		const r = el.getBoundingClientRect();
		clipScreens(el, r.left + (r.width * p) / 100);
		handle.current?.setAttribute('aria-valuenow', String(Math.round(p)));
		handle.current?.setAttribute('aria-valuetext', `${Math.round(p)}% light, ${100 - Math.round(p)}% dark`);
	};

	// Keep the cut on the divider while the devices move in, and whenever the stage changes size.
	useEffect(() => {
		const el = stage.current;
		if (!el) return;
		let frame = 0;
		const until = performance.now() + 1600;
		const follow = (now: number) => {
			apply(split.current);
			if (now < until) frame = requestAnimationFrame(follow);
		};
		frame = requestAnimationFrame(follow);
		const ro = new ResizeObserver(() => apply(split.current));
		ro.observe(el);
		return () => {
			cancelAnimationFrame(frame);
			ro.disconnect();
		};
	}, []);

	// One sweep out and back once the stage is in view and the devices have landed, then it rests. Never under reduced motion.
	useEffect(() => {
		const el = stage.current;
		if (!el || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		let frame = 0;
		let timer = 0;
		let start = 0;
		const tick = (now: number) => {
			if (touched.current) return;
			start ||= now;
			const t = (now - start) / SWEEP_MS;
			apply(REST + (t < 1 ? Math.sin(t * Math.PI * 2) * SWEEP : 0));
			if (t < 1) frame = requestAnimationFrame(tick);
		};
		const io = new IntersectionObserver(
			([entry]) => {
				if (!entry.isIntersecting) return;
				io.disconnect();
				timer = window.setTimeout(() => (frame = requestAnimationFrame(tick)), 1400);
			},
			{ threshold: 0.35 }
		);
		io.observe(el);
		return () => {
			io.disconnect();
			clearTimeout(timer);
			cancelAnimationFrame(frame);
		};
	}, []);

	const stop = () => {
		touched.current = true;
	};
	const fromPointer = (e: PointerEvent) => {
		const r = e.currentTarget.getBoundingClientRect();
		apply(((e.clientX - r.left) / r.width) * 100);
	};
	const onKey = (e: KeyboardEvent) => {
		const now = Math.round(split.current);
		const step = { ArrowLeft: -5, ArrowDown: -5, ArrowRight: 5, ArrowUp: 5, PageDown: -20, PageUp: 20 }[e.key];
		const to = e.key === 'Home' ? 0 : e.key === 'End' ? 100 : step === undefined ? null : now + step;
		if (to === null) return;
		e.preventDefault();
		stop();
		apply(to);
	};

	return (
		<div
			ref={stage}
			style={{ '--split': `${REST}%` } as CSSProperties}
			className={`relative cursor-ew-resize touch-pan-y [@media(scripting:none)]:cursor-auto select-none md:px-[3%] mx-auto mt-4 w-full md:max-w-[calc(clamp(240px,100svh-546px,540px)*1.73)] lg:mx-0 lg:mt-10 lg:w-auto lg:max-w-none lg:px-0 xl:-mr-[clamp(0px,calc((100vw-1120px)/2-24px),120px)]`}
			onPointerEnter={(e) => e.pointerType === 'mouse' && stop()}
			onPointerDown={(e) => {
				stop();
				dragging.current = true;
				e.currentTarget.setPointerCapture(e.pointerId);
				fromPointer(e);
			}}
			onPointerMove={(e) => dragging.current && fromPointer(e)}
			onPointerUp={() => (dragging.current = false)}
			onPointerCancel={() => (dragging.current = false)}
		>
			{/* Dark page only: a warm glow, so the dark half reads as lit screens rather than a hole in the page. */}
			<div aria-hidden className="pointer-events-none absolute inset-x-0 -inset-y-[8%] hidden bg-[radial-gradient(closest-side,color-mix(in_oklab,var(--color-fd-primary)_16%,transparent),transparent)] dark:block" />
			<div className="pointer-events-none relative">
				<Scene />
			</div>

			{/* The divider: a hairline across the whole stage and the knob that carries the slider role. */}
			<div className="pointer-events-none absolute -top-6 -bottom-6 left-[var(--split)] [@media(scripting:none)]:hidden w-0.5 -translate-x-1/2 bg-gradient-to-b from-transparent via-fd-primary to-transparent max-md:-top-3 max-md:-bottom-3" />
			<div
				ref={handle}
				role="slider"
				tabIndex={0}
				aria-label="Compare the light and dark theme"
				aria-orientation="horizontal"
				aria-valuemin={0}
				aria-valuemax={100}
				aria-valuenow={REST}
				aria-valuetext={`${REST}% light, ${100 - REST}% dark`}
				onKeyDown={onKey}
				onFocus={stop}
				className="absolute top-1/2 left-[var(--split)] flex size-12 max-md:top-[30%] [@media(scripting:none)]:hidden -translate-1/2 items-center justify-center rounded-full border-2 border-fd-primary bg-fd-card text-fd-primary shadow-lg outline-none focus-visible:ring-4 focus-visible:ring-fd-ring/40"
			>
				<ChevronsLeftRight className="size-5" aria-hidden />
			</div>

			{/* Which side is which: says what the knob does, also when nothing sweeps. Without JavaScript nothing splits, so the divider, knob and labels stay away. */}
			<span aria-hidden className={`pointer-events-none absolute -top-9 left-0 flex items-center gap-1.5 text-sm font-semibold text-fd-muted-foreground max-md:-top-7 max-md:left-[11%] md:left-[3%] lg:left-0 [@media(scripting:none)]:hidden`}>
				<Sun className="size-4" /> Light
			</span>
			<span aria-hidden className={`pointer-events-none absolute -top-9 right-0 flex items-center gap-1.5 text-sm font-semibold text-fd-muted-foreground max-md:-top-7 max-md:right-[11%] md:right-[3%] lg:right-0 [@media(scripting:none)]:hidden`}>
				Dark <Moon className="size-4" />
			</span>
		</div>
	);
}
