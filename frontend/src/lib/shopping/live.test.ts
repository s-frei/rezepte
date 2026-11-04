import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { followList } from './live';

class FakeSource {
	static last: FakeSource;
	static CLOSED = 2;
	listeners: Record<string, ((event: { data?: string }) => void)[]> = {};
	closed = false;
	readyState = 1;
	constructor(
		public url: string,
		public init?: EventSourceInit
	) {
		FakeSource.last = this;
	}
	addEventListener(type: string, fn: (event: { data?: string }) => void) {
		(this.listeners[type] ??= []).push(fn);
	}
	close() {
		this.closed = true;
		this.readyState = 2;
	}
	emit(type: string, data?: string) {
		for (const fn of this.listeners[type] ?? []) fn({ data });
	}
}
const Source = FakeSource as unknown as typeof EventSource;

describe('followList', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	it('calls onchange on changed', () => {
		const onchange = vi.fn();
		followList('l1', onchange, Source);
		expect(FakeSource.last.url).toBe('/api/v1/shopping-lists/l1/events');
		expect(FakeSource.last.init).toEqual({ withCredentials: true });
		FakeSource.last.emit('changed');
		vi.advanceTimersByTime(200);
		expect(onchange).toHaveBeenCalledTimes(1);
	});

	it("passes a burst's highest revision, none when an open joined it", () => {
		const onchange = vi.fn();
		followList('l1', onchange, Source);
		FakeSource.last.emit('changed', '{"revision":5}');
		FakeSource.last.emit('changed', '{"revision":3}');
		vi.advanceTimersByTime(200);
		expect(onchange).toHaveBeenLastCalledWith(5);
		FakeSource.last.emit('changed', '{"revision":7}');
		FakeSource.last.emit('open');
		vi.advanceTimersByTime(200);
		expect(onchange).toHaveBeenLastCalledWith(undefined);
		FakeSource.last.emit('changed', '{}');
		vi.advanceTimersByTime(200);
		expect(onchange).toHaveBeenLastCalledWith(undefined);
	});

	it('refetches on every open, the first included (a change may land before it)', () => {
		const onchange = vi.fn();
		followList('l1', onchange, Source);
		FakeSource.last.emit('open');
		vi.advanceTimersByTime(200);
		expect(onchange).toHaveBeenCalledTimes(1);
		FakeSource.last.emit('open');
		vi.advanceTimersByTime(200);
		expect(onchange).toHaveBeenCalledTimes(2);
	});

	it('refetches once when the stream is closed for good, so a 404 surfaces', () => {
		const onchange = vi.fn();
		followList('l1', onchange, Source);
		FakeSource.last.emit('error');
		vi.advanceTimersByTime(200);
		expect(onchange).not.toHaveBeenCalled(); // still CONNECTING: the browser retries
		FakeSource.last.readyState = 2;
		FakeSource.last.emit('error');
		vi.advanceTimersByTime(200);
		expect(onchange).toHaveBeenCalledTimes(1);
	});

	it('coalesces a burst into one call', () => {
		const onchange = vi.fn();
		followList('l1', onchange, Source);
		for (let i = 0; i < 5; i++) {
			FakeSource.last.emit('changed');
			vi.advanceTimersByTime(20);
		}
		vi.advanceTimersByTime(200);
		expect(onchange).toHaveBeenCalledTimes(1);
	});

	it('stop closes the source', () => {
		const onchange = vi.fn();
		const stop = followList('l1', onchange, Source);
		FakeSource.last.emit('changed');
		stop();
		vi.advanceTimersByTime(200);
		expect(FakeSource.last.closed).toBe(true);
		expect(onchange).not.toHaveBeenCalled();
	});
});
