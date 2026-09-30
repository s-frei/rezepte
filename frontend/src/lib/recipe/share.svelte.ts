import { toast } from 'svelte-sonner';
import { createShareLink } from '$lib/api/recipes';
import { m } from '$lib/paraglide/messages';
import { isStale, renewDelay } from './share';

/**
 * The address a recipe is shared by: the server's share link, whose token
 * shows the recipe in chat previews while link previews are on. Fetched ahead
 * and renewed before it runs out, so a tap can copy it or open the share
 * sheet at once - Safari refuses both once an await has spent the tap's user
 * activation. Timers are throttled in background tabs and suspended on
 * phones, so a page that becomes visible with a stale link renews it then.
 * Until a link arrives, or if fetching fails, the page's own address stands in.
 */
export class ShareLink {
	#path = $state<string | null>(null);
	#expiresAt: string | null = null;
	#timer: ReturnType<typeof setTimeout> | undefined;
	#destroyed = false;
	readonly #recipeId: string;

	constructor(recipeId: string) {
		this.#recipeId = recipeId;
		document.addEventListener('visibilitychange', this.#onVisible);
		void this.#load();
	}

	/** The absolute address to hand out. */
	get url(): string {
		// The server's path always starts at the root.
		return this.#path ? location.origin + this.#path : location.href;
	}

	destroy(): void {
		this.#destroyed = true;
		clearTimeout(this.#timer);
		document.removeEventListener('visibilitychange', this.#onVisible);
	}

	#onVisible = () => {
		if (document.visibilityState === 'visible' && isStale(this.#expiresAt, Date.now())) {
			void this.#load();
		}
	};

	async #load(): Promise<void> {
		clearTimeout(this.#timer);
		try {
			const link = await createShareLink(this.#recipeId);
			if (this.#destroyed) {
				return;
			}
			this.#path = link.path;
			this.#expiresAt = link.expiresAt;
		} catch {
			return;
		}
		const delay = renewDelay(this.#expiresAt, Date.now());
		if (delay !== null) {
			this.#timer = setTimeout(() => void this.#load(), delay);
		}
	}
}

/** Puts `text` on the clipboard and confirms it with `message`. */
export async function copyText(text: string, message: string): Promise<void> {
	await navigator.clipboard.writeText(text);
	toast.success(message);
}

export function copyLink(url: string): Promise<void> {
	return copyText(url, m.detail_link_copied());
}

/** The phone's share sheet where there is one, the clipboard elsewhere. */
export async function shareLink(title: string, url: string): Promise<void> {
	if (!navigator.share) {
		await copyLink(url);
		return;
	}
	try {
		await navigator.share({ title, url });
	} catch (e) {
		if (!(e instanceof DOMException && e.name === 'AbortError')) {
			await copyLink(url);
		}
	}
}

/** Whether this browser's share sheet takes files - Safari and Chrome on phones and macOS. */
export function canShareFiles(): boolean {
	if (typeof navigator.canShare !== 'function') {
		return false;
	}
	try {
		return navigator.canShare({ files: [new File([], 'card.png', { type: 'image/png' })] });
	} catch {
		return false;
	}
}

/** Whether an image can go on the clipboard (`ClipboardItem`). */
export function canCopyImage(): boolean {
	return typeof ClipboardItem !== 'undefined' && typeof navigator.clipboard?.write === 'function';
}

/** Saves `blob` as a download named `fileName`. */
export function downloadImage(blob: Blob, fileName: string): void {
	const url = URL.createObjectURL(blob);
	const link = document.createElement('a');
	link.href = url;
	link.download = fileName;
	link.click();
	// Revoked a moment later: some browsers start the download only after
	// the click handler has returned.
	setTimeout(() => URL.revokeObjectURL(url), 1000);
	toast.success(m.share_image_saved());
}

/** Puts the PNG on the clipboard, for pasting into a chat on a desktop. */
export async function copyImage(blob: Blob): Promise<void> {
	await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]);
	toast.success(m.share_image_copied());
}

/**
 * The phone's share sheet with the picture where it takes files, a
 * download elsewhere. `navigator.share` is the first thing awaited, so the
 * tap's user activation is still there for Safari; should Safari refuse
 * anyway (`NotAllowedError`), the picture is saved instead of lost. Any
 * other failure - a cancelled sheet (`AbortError`), a second call while one
 * is open (`InvalidStateError`) - does nothing: the person is looking at a
 * share sheet already, and a download on top would be a surprise.
 */
export async function shareImage(
	blob: Blob,
	fileName: string,
	title: string
): Promise<'shared' | 'saved' | 'aborted'> {
	const file = new File([blob], fileName, { type: 'image/png' });
	if (navigator.canShare?.({ files: [file] })) {
		try {
			await navigator.share({ files: [file], title });
			return 'shared';
		} catch (e) {
			if (!(e instanceof DOMException && e.name === 'NotAllowedError')) {
				return 'aborted';
			}
		}
	}
	downloadImage(blob, fileName);
	return 'saved';
}
