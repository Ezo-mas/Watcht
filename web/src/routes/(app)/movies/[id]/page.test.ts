import { describe, expect, it } from 'vitest';
import { isHttpError } from '@sveltejs/kit';
import { load } from './+page';

type LoadEvent = Parameters<typeof load>[0];

function event(id: string, fetchFn: typeof fetch): LoadEvent {
	return { params: { id }, fetch: fetchFn, depends: () => {} } as unknown as LoadEvent;
}

const answer = (status: number, body: unknown) =>
	(async () =>
		new Response(JSON.stringify(body), {
			status,
			headers: { 'Content-Type': 'application/json' }
		})) as unknown as typeof fetch;

/** Answers each request by its path. */
const byPath = (bodies: Record<string, unknown>) =>
	(async (input: RequestInfo | URL) => {
		const path = String(input);
		if (!(path in bodies)) return new Response('{}', { status: 500 });
		return new Response(JSON.stringify(bodies[path]), {
			status: 200,
			headers: { 'Content-Type': 'application/json' }
		});
	}) as unknown as typeof fetch;

const fail = async (p: unknown) => Promise.resolve(p).catch((e: unknown) => e);

describe('movie page load', () => {
	it("loads the movie and the user's watches of it by TMDB ID", async () => {
		const movie = { tmdb_id: 603, title: 'The Matrix' };
		const watches = [{ id: 7, watched_at: '2026-11-03T18:14:00Z', source: 'manual' }];
		const fetchFn = byPath({
			'/api/v1/movies/603': { movie },
			'/api/v1/movies/603/watches': { watches }
		});
		expect(await load(event('603', fetchFn))).toEqual({ movie, watches, watchesError: '' });
	});

	it("still shows the movie when the user's watches can't be loaded", async () => {
		const movie = { tmdb_id: 603, title: 'The Matrix' };
		// byPath answers anything it doesn't know with a 500.
		const fetchFn = byPath({ '/api/v1/movies/603': { movie } });
		expect(await load(event('603', fetchFn))).toMatchObject({
			movie,
			watches: null,
			watchesError: expect.any(String)
		});
	});

	it('is a 404 for an ID that is not a number, without asking the API', async () => {
		let called = false;
		const fetchFn = (async () => {
			called = true;
			return new Response('{}');
		}) as unknown as typeof fetch;
		const err = await fail(load(event('abc', fetchFn)));
		expect(isHttpError(err, 404)).toBe(true);
		expect(called).toBe(false);
	});

	it('is a 404 that names the ID when TMDB has no such movie', async () => {
		const err = await fail(
			load(event('999', answer(404, { error: { code: 'not_found', message: 'x' } })))
		);
		expect(isHttpError(err, 404)).toBe(true);
		expect(err).toMatchObject({ body: { message: 'TMDB has no movie with ID 999.' } });
	});

	it('explains a missing TMDB key', async () => {
		const body = { error: { code: 'tmdb_not_configured', message: 'x' } };
		const err = await fail(load(event('603', answer(503, body))));
		expect(isHttpError(err, 503)).toBe(true);
		expect(err).toMatchObject({
			body: { message: expect.stringContaining('WATCHT_TMDB_API_KEY') }
		});
	});
});
