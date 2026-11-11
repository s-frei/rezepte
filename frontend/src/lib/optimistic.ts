/**
 * Optimistic saves for one control whose requests can overlap. While the
 * newest request is in flight the control shows its value; once that request
 * settles, or the last one in flight does, the control shows the value the
 * server holds: the answer of the newest request that succeeded, or the
 * value from before the run when none did. `confirm` turns an answer into
 * that value, and runs only for an answer newer than the last confirmed one,
 * so it is the place to store the answer. A failure is passed on to the
 * caller, and `latest` tells whether a success is still the newest request.
 */
export function optimistic<T>(get: () => T, set: (value: T) => void) {
	let sent = 0;
	let pending = 0;
	let confirmedAt = 0;
	let confirmed: T;
	let newest: T;
	let open = false;

	return async function save<R>(
		next: T,
		request: () => Promise<R>,
		confirm: (answer: R) => T
	): Promise<{ answer: R; latest: boolean }> {
		if (pending === 0) {
			confirmed = get();
			confirmedAt = sent;
		}
		const seq = ++sent;
		pending++;
		newest = next;
		open = true;
		set(next);
		try {
			const answer = await request();
			if (seq > confirmedAt) {
				confirmedAt = seq;
				confirmed = confirm(answer);
			}
			return { answer, latest: seq === sent };
		} finally {
			pending--;
			if (seq === sent) open = false;
			set(open ? newest : confirmed);
		}
	};
}
