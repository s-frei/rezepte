import { save } from '#lib/storage.js';

/**
 * The app's other open tabs learn who is signed in now - after a switch, a
 * sign-in or a sign-out - because the session cookie changed under them: a
 * tab left as the person before would go on acting as them. A browser
 * without BroadcastChannel gets the news as a `storage` event instead, the
 * way the cook timers reach other tabs; the time makes every write a change.
 */
const NAME = 'rezepte-session';

// One channel for both ends: a message never reaches the object that sent it.
let channel: BroadcastChannel | null | undefined;
function open(): BroadcastChannel | null {
	channel ??= typeof BroadcastChannel === 'undefined' ? null : new BroadcastChannel(NAME);
	return channel;
}

/** Tells the other tabs that `userId`, or nobody, is signed in now. */
export function announceSession(userId: string | null): void {
	const ch = open();
	if (ch) ch.postMessage(userId);
	else save(NAME, JSON.stringify({ userId, at: Date.now() }));
}

/** Reloads this tab when another one announces someone other than `current()`. */
export function followSession(current: () => string | null): void {
	const follow = (userId: string | null) => {
		if (userId !== current()) location.reload();
	};
	const ch = open();
	if (ch) ch.onmessage = (e: MessageEvent<string | null>) => follow(e.data);
	else
		addEventListener('storage', (e) => {
			if (e.key === NAME && e.newValue) follow(JSON.parse(e.newValue).userId);
		});
}
