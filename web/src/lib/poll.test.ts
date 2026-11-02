import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { poll } from './poll';

// A season response, reduced to what polling looks at.
type Value = { pending: boolean };

function setup(values: Value[]) {
	const calls: AbortSignal[] = [];
	const load = vi.fn(async (signal: AbortSignal) => {
		calls.push(signal);
		return values[Math.min(calls.length - 1, values.length - 1)];
	});
	const onValue = vi.fn();
	const onTimeout = vi.fn();
	const onError = vi.fn();
	const cancel = poll<Value>({ load, done: (v) => !v.pending, onValue, onTimeout, onError });
	return { load, calls, onValue, onTimeout, onError, cancel };
}

const pending = { pending: true };
const ready = { pending: false };

describe('poll', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	it('stops after the first load when it is ready', async () => {
		const p = setup([ready]);
		await vi.runAllTimersAsync();
		expect(p.load).toHaveBeenCalledTimes(1);
		expect(p.onValue).toHaveBeenCalledWith(ready);
		expect(p.onTimeout).not.toHaveBeenCalled();
	});

	it('polls every 3 seconds until the value is ready', async () => {
		const p = setup([pending, pending, ready]);
		await vi.advanceTimersByTimeAsync(0);
		expect(p.load).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(2999);
		expect(p.load).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(1);
		expect(p.load).toHaveBeenCalledTimes(2);
		await vi.runAllTimersAsync();
		expect(p.load).toHaveBeenCalledTimes(3);
		expect(p.onValue).toHaveBeenLastCalledWith(ready);
		expect(p.onTimeout).not.toHaveBeenCalled();
	});

	it('gives up after two minutes', async () => {
		const p = setup([pending]);
		await vi.advanceTimersByTimeAsync(119_999);
		expect(p.onTimeout).not.toHaveBeenCalled();
		await vi.runAllTimersAsync();
		expect(p.onTimeout).toHaveBeenCalledTimes(1);
		// t = 0, 3, …, 120 s.
		expect(p.load).toHaveBeenCalledTimes(41);
		await vi.advanceTimersByTimeAsync(60_000);
		expect(p.load).toHaveBeenCalledTimes(41);
	});

	it('stops when cancelled between loads', async () => {
		const p = setup([pending]);
		await vi.advanceTimersByTimeAsync(0);
		p.cancel();
		await vi.advanceTimersByTimeAsync(30_000);
		expect(p.load).toHaveBeenCalledTimes(1);
		expect(p.onTimeout).not.toHaveBeenCalled();
	});

	it('aborts a load in flight and reports nothing after cancelling', async () => {
		let resolve: (v: Value) => void = () => {};
		let signal: AbortSignal | undefined;
		const onValue = vi.fn();
		const onError = vi.fn();
		const cancel = poll<Value>({
			load: (s) => {
				signal = s;
				return new Promise((r) => (resolve = r));
			},
			done: (v) => !v.pending,
			onValue,
			onTimeout: () => {},
			onError
		});
		cancel();
		expect(signal?.aborted).toBe(true);
		resolve(ready);
		await vi.runAllTimersAsync();
		expect(onValue).not.toHaveBeenCalled();
		expect(onError).not.toHaveBeenCalled();
	});

	it('does not report the error from a load it aborted', async () => {
		// Like fetch: aborting rejects the request. Seasons.svelte cancels on
		// a season switch, and that rejection must not show as an error.
		const onError = vi.fn();
		const cancel = poll<Value>({
			load: (signal) =>
				new Promise((_, reject) =>
					signal.addEventListener('abort', () => reject(new Error('aborted')))
				),
			done: (v) => !v.pending,
			onValue: () => {},
			onTimeout: () => {},
			onError
		});
		cancel();
		await vi.runAllTimersAsync();
		expect(onError).not.toHaveBeenCalled();
	});

	it('stops and reports a failed load', async () => {
		const onError = vi.fn();
		const load = vi.fn(async () => {
			throw new Error('down');
		});
		poll<Value>({ load, done: () => false, onValue: () => {}, onTimeout: () => {}, onError });
		await vi.runAllTimersAsync();
		expect(onError).toHaveBeenCalledWith(new Error('down'));
		expect(load).toHaveBeenCalledTimes(1);
	});
});
