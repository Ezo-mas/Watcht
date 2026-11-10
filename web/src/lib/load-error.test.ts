import { describe, expect, it } from 'vitest';
import { isHttpError } from '@sveltejs/kit';
import { ApiError } from './api/client';
import { catalogLoadError, loadError, optionalLoad } from './load-error';

const thrown = (f: () => void) => {
	try {
		f();
	} catch (e) {
		return e;
	}
	throw new Error('did not throw');
};

describe('loadError', () => {
	it("keeps an API error's status and shows its message", () => {
		const err = thrown(() => loadError(new ApiError(500, 'internal', 'the server failed')));
		expect(isHttpError(err, 500)).toBe(true);
		expect(err).toMatchObject({ body: { message: 'the server failed' } });
	});

	it('uses the message given for the code', () => {
		const e = new ApiError(429, 'rate_limited', 'x');
		const err = thrown(() => loadError(e, { rate_limited: 'Slow down.' }));
		expect(err).toMatchObject({ status: 429, body: { message: 'Slow down.' } });
	});

	it('is a 503 when the server could not be reached', () => {
		const e = new ApiError(0, 'network', "Can't reach the watcht server. Check your connection.");
		const err = thrown(() => loadError(e));
		expect(err).toMatchObject({ status: 503, body: { message: e.message } });
	});

	it('rethrows anything that is not an API error', () => {
		const bug = new TypeError('x is undefined');
		expect(thrown(() => loadError(bug))).toBe(bug);
	});
});

describe('catalogLoadError', () => {
	it('says what is missing on a 404', () => {
		const err = thrown(() =>
			catalogLoadError(new ApiError(404, 'not_found', 'x'), 'TMDB has no movie with ID 9.')
		);
		expect(err).toMatchObject({ status: 404, body: { message: 'TMDB has no movie with ID 9.' } });
	});

	it('says what kind of thing is missing, for the error page', () => {
		const err = thrown(() =>
			catalogLoadError(new ApiError(404, 'not_found', 'x'), 'TMDB has no show with ID 9.', 'show')
		);
		expect(err).toMatchObject({
			status: 404,
			body: { message: 'TMDB has no show with ID 9.', missing: 'show' }
		});
	});
});

describe('optionalLoad', () => {
	it('gives the value', async () => {
		expect(await optionalLoad(Promise.resolve([1]))).toEqual({ value: [1], error: '' });
	});

	it("gives null and the API's message instead of throwing", async () => {
		const failed = Promise.reject(new ApiError(500, 'internal', 'database is locked'));
		expect(await optionalLoad(failed)).toEqual({ value: null, error: 'database is locked' });
	});
});
