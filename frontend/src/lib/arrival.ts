import { match } from '$app/paths';
import type { UserColor } from '#lib/user/color.js';

/** Read and cleared by the inline script in app.html; keep the two equal. */
const KEY = 'rezepte-arrival';

/**
 * Just before the reload that ends a switch: hands the splash's ground to
 * the next page, whose app.html paints it before anything else (see
 * `html[data-arrival]` there). The value is the person's token as this page
 * resolved it: the fallback page links no stylesheet, so app.css and its
 * tokens only arrive with the bundle. Nothing under reduced motion, and
 * nothing for a `target` outside the app (the API's docs): that page never
 * reads the mark, and it would paint the next load in this tab instead.
 */
export async function markArrival(color: UserColor, target: string): Promise<void> {
	if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;
	if (!(await match(target))) return;
	const paint = getComputedStyle(document.documentElement).getPropertyValue(
		`--color-user-${color}`
	);
	try {
		sessionStorage.setItem(KEY, paint.trim());
	} catch {
		// No storage, no arrival: the app simply appears.
	}
}

/**
 * Once the signed-in app is on screen: the painted ground closes as a
 * circle into the person's avatar (the top bar's from md, the You tab
 * below), which pops once; app.css times it under `data-arrival="closing"`.
 * Without a visible avatar (cook mode, a page with no shell) it fades.
 */
export async function arrive(): Promise<void> {
	const root = document.documentElement;
	if (root.dataset.arrival !== '') return;
	const target = [...document.querySelectorAll('[data-arrival-target]')].find(
		(el) => el.getBoundingClientRect().width > 0
	);
	// The circle closed onto an empty ring while the photo still loaded;
	// waited for, capped so the ground never holds the app back for long.
	const photo = target?.querySelector('img');
	if (photo) {
		await Promise.race([photo.decode().catch(() => {}), new Promise((r) => setTimeout(r, 300))]);
	}
	if (target) {
		const box = target.getBoundingClientRect();
		const x = box.left + box.width / 2;
		const y = box.top + box.height / 2;
		const r = Math.hypot(Math.max(x, innerWidth - x), Math.max(y, innerHeight - y));
		root.style.setProperty('--arrival-at', `${x}px ${y}px`);
		root.style.setProperty('--arrival-r', `${r}px`);
	}
	root.dataset.arrival = target ? 'closing' : 'fading';
	setTimeout(() => {
		delete root.dataset.arrival;
		for (const p of ['--arrival-paint', '--arrival-at', '--arrival-r'])
			root.style.removeProperty(p);
	}, 700);
}
