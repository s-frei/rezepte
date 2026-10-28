import { browser } from '$app/environment';
import { installOffer, isIos, readDismissed, writeDismissed } from './install';

/** Chromium's `beforeinstallprompt` event; no DOM typing ships for it. */
interface InstallPrompt extends Event {
	prompt(): Promise<unknown>;
	userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

/** Facts about the page that do not change while it is open. */
export const secure = browser && window.isSecureContext;
export const standalone =
	browser &&
	(window.matchMedia('(display-mode: standalone)').matches ||
		(navigator as Navigator & { standalone?: boolean }).standalone === true);
const ios = browser && isIos(navigator.userAgent, navigator.platform, navigator.maxTouchPoints);

/**
 * The captured prompt, whether the app got installed in this page view, the
 * banner's dismissal and the iPhone sheet. `dismissed` is kept here as well
 * as in storage, so × works where storage throws.
 */
export const install = $state({
	// One that fired before this module loaded was kept by app.html.
	prompt: (browser
		? ((window as Window & { __installPrompt?: InstallPrompt }).__installPrompt ?? null)
		: null) as InstallPrompt | null,
	installed: false,
	dismissed: browser && readDismissed(() => window.localStorage),
	iosSheet: false
});

// `beforeinstallprompt` can fire before any component mounts, so the
// listeners live at module level; the root layout imports this module early,
// and app.html covers the time before that.
if (browser) {
	window.addEventListener('beforeinstallprompt', (event) => {
		event.preventDefault();
		install.prompt = event as InstallPrompt;
	});
	window.addEventListener('appinstalled', () => {
		install.installed = true;
		install.prompt = null;
	});
}

/** Whether and how this device can install now; reads live state, so it works in `$derived`. */
export const offer = () =>
	installOffer({
		secure,
		standalone,
		installed: install.installed,
		ios,
		hasPrompt: install.prompt !== null
	});

/** The browser's install dialog, or the iPhone steps. */
export async function startInstall(): Promise<void> {
	const kind = offer();
	if (kind === 'ios') {
		install.iosSheet = true;
	} else if (kind === 'prompt' && install.prompt) {
		const prompt = install.prompt;
		try {
			await prompt.prompt();
			await prompt.userChoice;
		} catch {
			// A prompt that cannot be shown is spent all the same.
		} finally {
			// A prompt can be shown only once; Chromium fires a new event if it
			// offers the install again. Accepting is not installing: only
			// `appinstalled` hides the hints for good.
			install.prompt = null;
		}
	}
}

export function dismissBanner(): void {
	install.dismissed = true;
	writeDismissed(() => window.localStorage);
}
