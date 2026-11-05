import { describe, expect, it } from 'vitest';
import { latestOnly } from './latest';

/** A promise and the functions that settle it. */
function deferred<T>() {
	let resolve!: (v: T) => void;
	let reject!: (e: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

describe('latestOnly', () => {
	it('passes a lone request through', async () => {
		const track = latestOnly<number>();
		expect(await track(Promise.resolve(1))).toEqual({ current: true, value: 1 });
	});

	it('drops an older answer that arrives after a newer one', async () => {
		const track = latestOnly<string>();
		const first = deferred<string>();
		const second = deferred<string>();
		const a = track(first.promise);
		const b = track(second.promise);
		second.resolve('new');
		first.resolve('old');
		expect(await b).toEqual({ current: true, value: 'new' });
		expect(await a).toEqual({ current: false });
	});

	it('drops an older answer that arrives first', async () => {
		const track = latestOnly<string>();
		const first = deferred<string>();
		const second = deferred<string>();
		const a = track(first.promise);
		const b = track(second.promise);
		first.resolve('old');
		expect(await a).toEqual({ current: false });
		second.resolve('new');
		expect(await b).toEqual({ current: true, value: 'new' });
	});

	it('ignores an older failure, and throws the latest one', async () => {
		const track = latestOnly<string>();
		const first = deferred<string>();
		const a = track(first.promise);
		const b = track(Promise.reject(new Error('down')));
		first.reject(new Error('old'));
		expect(await a).toEqual({ current: false });
		await expect(b).rejects.toThrow('down');
	});
});
