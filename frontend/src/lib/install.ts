/**
 * How this device can install Rezepte right now: the browser's own dialog
 * from a captured `beforeinstallprompt` (Android, desktop Chrome), the
 * Safari steps on an iPhone or iPad, which has no such event, or not at all.
 * Every install hint starts from this one answer and adds its own condition.
 */
export type InstallOffer = 'prompt' | 'ios' | null;

export function installOffer(env: {
	secure: boolean;
	standalone: boolean;
	installed: boolean;
	ios: boolean;
	hasPrompt: boolean;
}): InstallOffer {
	if (!env.secure || env.standalone || env.installed) return null;
	if (env.hasPrompt) return 'prompt';
	return env.ios ? 'ios' : null;
}

/**
 * The settings card shows the offer, or, for the owner on plain HTTP, why
 * there is none: only the owner can put the instance behind HTTPS.
 */
export type AppCardState = 'prompt' | 'ios' | 'https' | null;

export function appCardState(
	offer: InstallOffer,
	env: { secure: boolean; standalone: boolean; owner: boolean }
): AppCardState {
	if (offer) return offer;
	return !env.secure && !env.standalone && env.owner ? 'https' : null;
}

/**
 * iPadOS Safari asks for desktop sites by default and then calls itself a
 * Mac; only the touch points give it away, since no Mac has a touch screen.
 */
export function isIos(ua: string, platform: string, maxTouchPoints: number): boolean {
	return /iPhone|iPad|iPod/.test(ua) || (platform === 'MacIntel' && maxTouchPoints > 1);
}

export const DISMISSED_STORAGE_KEY = 'rezepte-install-banner-dismissed';
