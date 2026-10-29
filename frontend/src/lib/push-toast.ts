import { toast } from 'svelte-sonner';
import { goto } from '$app/navigation';
import { parseToastMessage } from '$lib/push-worker';
import { m } from '$lib/paraglide/messages';

// Longer than the default, so there is time to reach the button.
const duration = 8000;

/**
 * A notification push that arrives while this window is visible also shows
 * here as a toast; "Open" goes to its page and closes the notification, so it
 * does not wait in the notification centre for what was just read. Returns the
 * listener's removal.
 */
export function listenForPushToasts(): () => void {
	if (!('serviceWorker' in navigator)) {
		return () => {};
	}
	const onMessage = (event: MessageEvent) => {
		const msg = parseToastMessage(event.data);
		if (!msg) {
			return;
		}
		toast(msg.title, {
			description: msg.body,
			duration,
			// A pill, not the muted text of the other toasts' actions: this one
			// is the way to the page, so it has to read as a button.
			classes: {
				actionButton:
					'rounded-pill! border! border-inverse-muted! bg-transparent! px-3.5! py-1.5! font-semibold! text-inverse-foreground! hover:bg-inverse-foreground/10!'
			},
			action: {
				label: m.push_toast_open(),
				onClick: () => {
					// A path inside the app, checked by parseToastMessage.
					// eslint-disable-next-line svelte/no-navigation-without-resolve
					void goto(msg.url);
					void closeNotification(msg.tag);
				}
			}
		});
	};
	navigator.serviceWorker.addEventListener('message', onMessage);
	return () => navigator.serviceWorker.removeEventListener('message', onMessage);
}

async function closeNotification(tag: string): Promise<void> {
	try {
		const reg = await navigator.serviceWorker.getRegistration();
		for (const n of (await reg?.getNotifications({ tag })) ?? []) {
			n.close();
		}
	} catch {
		// the notification stays; nothing else depends on it
	}
}
