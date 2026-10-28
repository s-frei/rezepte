import { goto } from '$app/navigation';

// The Launch Handler API has no dependency-free types yet.
type LaunchParams = { targetURL?: string };
type LaunchQueue = { setConsumer(consumer: (params: LaunchParams) => void): void };

// The launch that opened this window is already handled by the router; only
// later ones, into an open window, arrive through the consumer as news.
const initialHref = typeof location === 'undefined' ? '' : location.href;

export function isFreshLaunch(target: string, initial: string): boolean {
	return new URL(target).href !== new URL(initial).href;
}

/** Share → Rezepte into an already open window (`launch_handler:
 * focus-existing`): navigates there instead of reloading it. The navigation
 * passes the editor's leave guard, so unsaved changes ask first. */
export function listenForLaunches(): void {
	const queue = (window as { launchQueue?: LaunchQueue }).launchQueue;
	queue?.setConsumer((params) => {
		if (!params.targetURL || !isFreshLaunch(params.targetURL, initialHref)) return;
		const target = new URL(params.targetURL);
		// A path from the same origin, built by the OS from the manifest.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		void goto(target.pathname + target.search);
	});
}
