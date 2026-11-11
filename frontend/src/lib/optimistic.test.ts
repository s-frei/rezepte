import { describe, expect, it } from 'vitest';
import { optimistic } from './optimistic';

/** A request the test settles by hand. */
function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (error: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

function control(initial: string) {
	let shown = initial;
	const save = optimistic(
		() => shown,
		(value: string) => (shown = value)
	);
	return {
		shown: () => shown,
		send(next: string) {
			const request = deferred<string>();
			const result = save(
				next,
				() => request.promise,
				(answer) => answer
			);
			result.catch(() => {});
			return { ...request, result };
		}
	};
}

describe('optimistic', () => {
	it('shows the answer of a plain success', async () => {
		const c = control('off');
		const a = c.send('on');
		expect(c.shown()).toBe('on');
		a.resolve('on');
		expect(await a.result).toEqual({ answer: 'on', latest: true });
		expect(c.shown()).toBe('on');
	});

	it('ends on an overtaken success when the newer request fails', async () => {
		const c = control('x');
		const a = c.send('a');
		const b = c.send('b');
		a.resolve('a');
		expect(await a.result).toEqual({ answer: 'a', latest: false });
		expect(c.shown()).toBe('b');
		b.reject(new Error('refused'));
		await expect(b.result).rejects.toThrow('refused');
		expect(c.shown()).toBe('a');
	});

	it('ends on the value from before the run when both fail', async () => {
		const c = control('x');
		const a = c.send('a');
		const b = c.send('b');
		a.reject(new Error('a'));
		await expect(a.result).rejects.toThrow();
		expect(c.shown()).toBe('b');
		b.reject(new Error('b'));
		await expect(b.result).rejects.toThrow();
		expect(c.shown()).toBe('x');
	});

	it('takes an older success that lands after the newer failure', async () => {
		const c = control('x');
		const a = c.send('a');
		const b = c.send('b');
		b.reject(new Error('b'));
		await expect(b.result).rejects.toThrow();
		expect(c.shown()).toBe('x');
		a.resolve('a');
		await a.result;
		expect(c.shown()).toBe('a');
	});

	it('stays on the newest success when an older one lands late', async () => {
		const c = control('x');
		const a = c.send('a');
		const b = c.send('b');
		b.resolve('b');
		expect(await b.result).toEqual({ answer: 'b', latest: true });
		a.resolve('a');
		expect(await a.result).toEqual({ answer: 'a', latest: false });
		expect(c.shown()).toBe('b');
	});
});
