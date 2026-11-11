/**
 * The app's other open tabs learn who is signed in now - after a switch, a
 * sign-in or a sign-out - because the session cookie changed under them: a
 * tab left as the person before would go on acting as them.
 */
// One channel for both ends: a message never reaches the object that sent it.
let channel: BroadcastChannel | undefined;
const open = () => (channel ??= new BroadcastChannel('rezepte-session'));

/** Tells the other tabs that `userId`, or nobody, is signed in now. */
export function announceSession(userId: string | null): void {
	open().postMessage(userId);
}

/** Reloads this tab when another one announces someone other than `current()`. */
export function followSession(current: () => string | null): void {
	open().onmessage = (e: MessageEvent<string | null>) => {
		if (e.data !== current()) location.reload();
	};
}
