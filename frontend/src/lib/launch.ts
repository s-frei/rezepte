import { goto } from '$app/navigation';

// The Launch Handler API has no dependency-free types yet.
type LaunchParams = { targetURL?: string };
type LaunchQueue = { setConsumer(consumer: (params: LaunchParams) => void): void };

// The launch that opened this window is already handled by the router; only
// later ones, into an open window, arrive through the consumer as news. Only
// the first launch can be that one, so it is forgotten after the first look.
export function launchFilter(initial: string): (target: string) => boolean {
	let first = initial;
	return (target) => {
		const seen = first;
		first = '';
		return new URL(target).href !== seen;
	};
}

/** Share → Rezepte into an already open window (`launch_handler:
 * focus-existing`): navigates there instead of reloading it. The navigation
 * passes the editor's leave guard, so unsaved changes ask first. */
export function listenForLaunches(): void {
	const queue = (window as { launchQueue?: LaunchQueue }).launchQueue;
	const isFresh = launchFilter(location.href);
	queue?.setConsumer((params) => {
		if (!params.targetURL || !isFresh(params.targetURL)) return;
		const target = new URL(params.targetURL);
		// A path from the same origin, built by the OS from the manifest.
		void goto(target.pathname + target.search);
	});
}
