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

export async function copyLink(url: string): Promise<void> {
	await navigator.clipboard.writeText(url);
	toast.success(m.detail_link_copied());
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
