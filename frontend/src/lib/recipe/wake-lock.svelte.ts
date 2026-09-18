/**
 * Screen Wake Lock for cooking mode: keeps the phone from dimming while a
 * recipe is on screen. `active` is `$state` so the page can show the
 * "Bildschirm bleibt an" caption only while a lock is really held.
 *
 * The API exists only in secure contexts (https or localhost) and the
 * request rejects while the tab is hidden; both cases leave `active`
 * false without throwing. The browser releases the lock itself when the
 * tab goes to the background - the page re-acquires on `visibilitychange`.
 */
async function releaseQuietly(sentinel: WakeLockSentinel): Promise<void> {
	try {
		await sentinel.release();
	} catch {
		// Already released by the browser.
	}
}

export class ScreenWakeLock {
	active = $state(false);
	#sentinel: WakeLockSentinel | null = null;
	/**
	 * Bumped by every `acquire` and `release`. A request that only resolves
	 * after the page left cooking mode - or after a second `acquire` started
	 * its own request - would otherwise install a sentinel nobody releases
	 * and the screen would stay on; the generation check drops it instead.
	 */
	#gen = 0;

	async acquire(): Promise<void> {
		if (this.#sentinel !== null || typeof navigator === 'undefined' || !('wakeLock' in navigator)) {
			return;
		}
		const gen = ++this.#gen;
		try {
			const sentinel = await navigator.wakeLock.request('screen');
			if (gen !== this.#gen) {
				await releaseQuietly(sentinel);
				return;
			}
			this.#sentinel = sentinel;
			this.active = true;
			sentinel.addEventListener('release', () => {
				if (this.#sentinel === sentinel) {
					this.#sentinel = null;
					this.active = false;
				}
			});
		} catch {
			// Denied (low battery, hidden tab, policy): cooking mode works without it.
			if (gen === this.#gen) {
				this.active = false;
			}
		}
	}

	async release(): Promise<void> {
		this.#gen++;
		const sentinel = this.#sentinel;
		this.#sentinel = null;
		this.active = false;
		if (sentinel !== null) {
			await releaseQuietly(sentinel);
		}
	}
}
