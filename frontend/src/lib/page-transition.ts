import type { OnNavigate } from '@sveltejs/kit';

const OVERVIEW = '/';
const DETAIL = '/recipes/[slug]';

/** The view-transition name the overview card's photo and the detail cover share. */
const COVER = 'recipe-cover';

/**
 * The photo that stands for the recipe on this page: the card's photo on the
 * overview (`data-cover` carries the slug), the gallery's cover on the
 * detail page. Only a rendered one counts - the detail page keeps a phone
 * and a desktop gallery and hides one - and on the overview only one on
 * screen: a card scrolled away has nowhere visible to glide from or to.
 */
function cover(route: string | null | undefined, slug: string): HTMLElement | null {
	const selector =
		route === OVERVIEW
			? `[data-cover="${CSS.escape(slug)}"]`
			: route === DETAIL
				? '[data-cover]'
				: null;
	if (!selector) {
		return null;
	}
	for (const el of document.querySelectorAll<HTMLElement>(selector)) {
		const box = el.getBoundingClientRect();
		if (box.width > 0 && box.bottom > 0 && box.top < window.innerHeight) {
			return el;
		}
	}
	return null;
}

const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Waits for the new page's photo to be there and painted: the overview
 * fetches its cards after it mounts, and a cover still decoding would glide
 * in as an empty box. The old page stays on screen meanwhile, so the wait
 * is capped at what still reads as an answer to the tap. Polled with timers:
 * the browser does not render, and so runs no animation frames, while a
 * view transition's update is pending.
 */
async function arrived(route: string, slug: string): Promise<HTMLElement | null> {
	const deadline = performance.now() + 400;
	let el = cover(route, slug);
	while (!el && performance.now() < deadline) {
		await wait(16);
		el = cover(route, slug);
	}
	const img = el instanceof HTMLImageElement ? el : el?.querySelector('img');
	if (img) {
		await Promise.race([img.decode().catch(() => {}), wait(deadline - performance.now())]);
	}
	return el;
}

/**
 * Turns a navigation between two pages into a view transition: the old page
 * fades into the new one (`data-page-transition` times it in app.css), and
 * between the overview and a recipe the card's photo becomes the recipe's
 * cover, there and back. Returned from `onNavigate`, so SvelteKit swaps the
 * page inside the transition's update.
 *
 * Not for a change of the query on the same page (the overview's filters
 * have their own), not under reduced motion, and not for a back or forward
 * on a touch screen, whose swipe already draws a page sliding away.
 */
export function pageTransition(navigation: OnNavigate): Promise<void> | undefined {
	const from = navigation.from;
	const to = navigation.to;
	if (
		!('startViewTransition' in document) ||
		!from?.route.id ||
		!to?.route.id ||
		from.route.id === to.route.id ||
		window.matchMedia('(prefers-reduced-motion: reduce)').matches ||
		(navigation.type === 'popstate' && window.matchMedia('(pointer: coarse)').matches)
	) {
		return undefined;
	}
	const slug = (from.route.id === DETAIL ? from : to).params?.slug;
	const pair =
		slug !== undefined &&
		((from.route.id === OVERVIEW && to.route.id === DETAIL) ||
			(from.route.id === DETAIL && to.route.id === OVERVIEW));
	const named: HTMLElement[] = [];
	const name = (el: HTMLElement | null) => {
		if (el) {
			el.style.setProperty('view-transition-name', COVER);
			named.push(el);
		}
	};

	return new Promise((resolve) => {
		if (pair) {
			name(cover(from.route.id, slug));
		}
		const root = document.documentElement;
		root.dataset.pageTransition = '';
		const transition = document.startViewTransition(async () => {
			resolve();
			await navigation.complete;
			// Only a pair the old page started can be finished on the new one.
			if (named.length) {
				named[0].style.removeProperty('view-transition-name');
				name(await arrived(to.route.id!, slug!));
			}
		});
		transition.finished
			.catch(() => {})
			.finally(() => {
				delete root.dataset.pageTransition;
				for (const el of named) {
					el.style.removeProperty('view-transition-name');
				}
			});
	});
}
