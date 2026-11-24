import { mkdirSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import type { Locator, Page } from '@playwright/test';

// A film is the real app driven by a script: Playwright's screencast hands
// over every frame the compositor paints (JPEG, with its timestamp), and
// scripts/film-encode.ts turns them into public/film/<name>.{mp4,jpg}.
// Headless Chromium paints no mouse pointer, so an overlay draws one where
// the real mouse is; the mouse itself moves in small steps, so hover states
// show as a person's pointer would cause them.

/** Where a film's frames wait for scripts/film-encode.ts. */
export const FRAMES = path.join(__dirname, '../test-results/film-frames');

type Target = Locator | { x: number; y: number };
/** Where on a locator the pointer lands: its center, or its left part (an icon, the start of a label) so the word stays readable. */
type Aim = 'center' | 'left';

/** Draws the pointer at the mouse and a ripple on every press; survives reloads through sessionStorage. */
function cursorOverlay() {
	const KEY = 'film-cursor';
	const el = document.createElement('div');
	el.style.cssText = 'position:fixed;left:0;top:0;width:0;height:0;z-index:2147483647;pointer-events:none;will-change:transform';
	el.innerHTML =
		'<svg width="28" height="28" viewBox="0 0 28 28" style="position:absolute;left:-5px;top:-3px;filter:drop-shadow(0 1.5px 2px rgb(0 0 0 / .35))">' +
		'<path d="M6 3.5v18.2l4.6-4.3 3.1 7.3 3.3-1.4-3.1-7.2h6.3z" fill="#1b1b1b" stroke="#fff" stroke-width="1.7" stroke-linejoin="round"/></svg>';
	const move = (x: number, y: number) => {
		el.style.transform = `translate(${x}px, ${y}px)`;
		sessionStorage.setItem(KEY, JSON.stringify([x, y]));
	};
	const [x, y] = JSON.parse(sessionStorage.getItem(KEY) ?? '[-60,-60]');
	move(x, y);
	addEventListener('mousemove', (e) => move(e.clientX, e.clientY), { capture: true, passive: true });
	addEventListener(
		'mousedown',
		(e) => {
			const r = document.createElement('div');
			r.style.cssText = `position:fixed;left:${e.clientX - 22}px;top:${e.clientY - 22}px;width:44px;height:44px;border-radius:50%;z-index:2147483646;pointer-events:none;background:rgb(0 0 0 / .16);box-shadow:0 0 0 2px rgb(255 255 255 / .85), 0 0 0 3px rgb(0 0 0 / .25)`;
			document.documentElement.append(r);
			r.animate([{ transform: 'scale(.2)', opacity: 1 }, { transform: 'scale(1)', opacity: 0 }], { duration: 480, easing: 'ease-out' }).finished.then(() => r.remove());
		},
		{ capture: true, passive: true }
	);
	const mount = () => document.documentElement.append(el);
	if (document.documentElement) mount();
	else addEventListener('DOMContentLoaded', mount);
}

const ease = (t: number) => (t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2);
const easeOut = (t: number) => 1 - (1 - t) ** 3;

/** How long a still frame may stand during a squashed wait. */
const SQUASH_KEEP = 150;

/**
 * Prepares `page` for a film called `name` (add before the first `goto`):
 * the pointer overlay, and the camera, which `roll` starts on a page that is
 * ready and `cut` stops.
 */
export async function film(page: Page, name: string) {
	await page.addInitScript(cursorOverlay);
	const dir = path.join(FRAMES, name);
	// Each frame's browser timestamp and when it arrived here; squashed waits and marks are in wall time.
	const frames: { ts: number; arrival: number }[] = [];
	const squashes: [number, number][] = [];
	const skips: [number, number][] = [];
	const marks: Record<string, number> = {};
	let at = { x: -60, y: -60 };
	const wait = (until: number) => page.waitForTimeout(Math.max(0, until - Date.now()));

	const f = {
		/** Starts recording; the first frame is the page as it is now. */
		async roll() {
			rmSync(dir, { recursive: true, force: true });
			mkdirSync(dir, { recursive: true });
			// Chromium's screencast paints at CSS pixels whatever the deviceScaleFactor, so the viewport is the film's size.
			await page.screencast.start({
				size: page.viewportSize()!,
				quality: 95,
				onFrame: ({ data, timestamp }) => {
					writeFileSync(path.join(dir, `${String(frames.length).padStart(5, '0')}.jpg`), data);
					frames.push({ ts: timestamp, arrival: Date.now() });
				}
			});
		},

		/**
		 * Stops recording and writes the frame list (frames.txt, each frame with
		 * its duration, squashed waits cut down) and the marks in film seconds
		 * (marks.json, with `duration`).
		 */
		async cut() {
			const end = Date.now();
			await page.screencast.stop();
			if (!frames.length) throw new Error(`film ${name}: no frames`);
			// Frame timestamps are milliseconds on the browser's own clock; the
			// smallest arrival delay maps them onto this process's clock.
			const offset = Math.min(...frames.map((f) => f.arrival - f.ts));
			// Inside a squashed wait only stillness is cut: a frame that stands
			// longer than SQUASH_KEEP is shortened to it, while frames that keep
			// coming (a transition, a spinner) all stay.
			const wall = [...frames.map((f) => f.ts + offset), end];
			// A skipped stretch goes entirely.
			const cuts = [
				...frames.flatMap((_, i) =>
					squashes.map(([s, e]) => [Math.max(wall[i] + SQUASH_KEEP, s), Math.min(wall[i + 1], e)]).filter(([a, b]) => b > a)
				),
				...skips
			];
			const onFilm = (t: number) => t - cuts.reduce((sum, [s, e]) => sum + Math.max(0, Math.min(t, e) - s), 0);
			const t0 = onFilm(frames[0].ts + offset);
			const startOf = (i: number) => (i < frames.length ? onFilm(frames[i].ts + offset) : onFilm(end)) - t0;
			const lines: string[] = [];
			let last = '';
			frames.forEach((_, i) => {
				const ms = startOf(i + 1) - startOf(i);
				if (ms < 1) return;
				last = `file '${String(i).padStart(5, '0')}.jpg'`;
				lines.push(`${last}\nduration ${(ms / 1000).toFixed(4)}`);
			});
			// The concat demuxer only honors the last duration when the file repeats.
			lines.push(last);
			writeFileSync(path.join(dir, 'frames.txt'), lines.join('\n') + '\n');
			const secs = (t: number) => Math.round(onFilm(t) - t0) / 1000;
			const out = Object.fromEntries(Object.entries(marks).map(([k, t]) => [k, secs(t)]));
			writeFileSync(path.join(dir, 'marks.json'), JSON.stringify({ ...out, duration: secs(end) }, null, '\t') + '\n');
		},

		/** Runs `wait` (a load, a render); a still screen during it shows for 150 ms at most, motion stays. */
		async squash<T>(wait: () => Promise<T>): Promise<T> {
			const start = Date.now();
			try {
				return await wait();
			} finally {
				squashes.push([start, Date.now()]);
			}
		},

		/** Runs `change` and cuts all of it from the film: the shot jumps from before it to after it (a sheet that renders its card, a close). */
		async skip<T>(change: () => Promise<T>): Promise<T> {
			const start = Date.now();
			try {
				return await change();
			} finally {
				skips.push([start, Date.now()]);
			}
		},

		/**
		 * Waits for `landed` (squashed: a slow load never shows), lets the
		 * change's animations finish, then holds so the viewer has `ms` from
		 * the moment it landed - and never less than half of it after the
		 * animations, so a long transition still leaves time to read. `label`
		 * marks the settled moment.
		 */
		async land(landed: () => Promise<unknown>, ms: number, label?: string) {
			await f.squash(landed);
			const t = Date.now();
			await f.settle();
			if (label) f.mark(label);
			await f.hold(Math.max(ms * 0.5, ms - (Date.now() - t)));
		},

		/** Names this moment in marks.json, so a review can find a beat by name. */
		mark(label: string) {
			marks[label] = Date.now();
		},

		/** Waits until animations that end have finished (0.8 s at most) and the page has stopped scrolling. */
		async settle() {
			await page.evaluate(async () => {
				// A running timer's ring animates forever; only animations that end are waited for.
				const ending = document.getAnimations().filter((a) => a.effect?.getComputedTiming().iterations !== Infinity);
				await Promise.race([Promise.allSettled(ending.map((a) => a.finished)), new Promise((r) => setTimeout(r, 800))]);
				const frame = () => new Promise((r) => requestAnimationFrame(r));
				for (let y = -1; y !== scrollY; ) {
					y = scrollY;
					await frame();
					await frame();
				}
			});
		},

		/** Moves the mouse to `target` (its center, or its left part with `aim`) or a point along a slight arc, eased, over `ms`. */
		async glide(target: Target, ms = 700, aim: Aim = 'center') {
			let to = target as { x: number; y: number };
			if (!('x' in target)) {
				const b = await target.boundingBox({ timeout: 10_000 });
				const x = (b: { x: number; width: number }) => (aim === 'left' ? b.x + Math.min(20, b.width / 4) : b.x + b.width / 2);
				to = b ? { x: x(b), y: b.y + b.height / 2 } : { x: -1, y: -1 };
				const vp = page.viewportSize()!;
				if (to.x < 0 || to.y < 0 || to.x > vp.width || to.y > vp.height) throw new Error(`glide: ${target} is not on screen (scroll to it first)`);
			}
			const from = at;
			const dx = to.x - from.x;
			const dy = to.y - from.y;
			const steps = Math.max(1, Math.round(ms / 16));
			const t0 = Date.now();
			for (let i = 1; i <= steps; i++) {
				const e = ease(i / steps);
				const bend = Math.sin(Math.PI * e) * 0.08;
				await page.mouse.move(from.x + dx * e - dy * bend, from.y + dy * e + dx * bend);
				await wait(t0 + (i * ms) / steps);
			}
			at = to;
		},

		/** Presses and releases the left button, after gliding to `target` if given. */
		async click(target?: Target, glideMs?: number, aim?: Aim) {
			if (target) {
				await f.glide(target, glideMs, aim);
				await f.hold(80);
			}
			await page.mouse.down();
			await f.hold(60);
			await page.mouse.up();
		},

		/** Waits, recording. */
		hold: (ms: number) => page.waitForTimeout(ms),

		/** Scrolls by `dy` pixels with the wheel under the pointer, fast first like a trackpad, over `ms`; then settles. */
		async scroll(dy: number, ms = 800) {
			const steps = Math.max(1, Math.round(ms / 16));
			const t0 = Date.now();
			let done = 0;
			for (let i = 1; i <= steps; i++) {
				const next = Math.round(dy * easeOut(i / steps));
				if (next !== done) await page.mouse.wheel(0, next - done);
				done = next;
				await wait(t0 + (i * ms) / steps);
			}
			await f.settle();
		},

		/** Scrolls until `target`'s top sits `top` px below the viewport's top. */
		async scrollTo(target: Locator, top: number, ms = 800) {
			const b = await target.boundingBox({ timeout: 10_000 });
			if (!b) throw new Error(`scrollTo: ${target} has no box`);
			await f.scroll(Math.round(b.y - top), ms);
		},

		/** Types into the focused field, one key every `perKeyMs`. */
		type: (text: string, perKeyMs = 70) => page.keyboard.type(text, { delay: perKeyMs })
	};
	return f;
}
