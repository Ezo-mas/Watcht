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

describe('show page load', () => {
	it('loads the show and its watched episodes by TMDB ID', async () => {
		const show = { tmdb_id: 1396, name: 'Breaking Bad', seasons: [] };
		const episodes = [{ tmdb_id: 62085, plays: 2, last_watched_at: '2026-11-03T18:14:00Z' }];
		const fetchFn = byPath({
			'/api/v1/shows/1396': { show },
			'/api/v1/shows/1396/watched': { episodes }
		});
		expect(await load(event('1396', fetchFn))).toEqual({
			show,
			watched: episodes,
			watchedError: ''
		});
	});

	it("still shows the show when the watched episodes can't be loaded", async () => {
		const show = { tmdb_id: 1396, name: 'Breaking Bad', seasons: [] };
		const fetchFn = byPath({ '/api/v1/shows/1396': { show } });
		expect(await load(event('1396', fetchFn))).toMatchObject({
			show,
			watched: null,
			watchedError: expect.any(String)
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

	it('is a 404 that names the ID when TMDB has no such show', async () => {
		const err = await fail(
			load(event('999', answer(404, { error: { code: 'not_found', message: 'x' } })))
		);
		expect(isHttpError(err, 404)).toBe(true);
		expect(err).toMatchObject({ body: { message: 'TMDB has no show with ID 999.' } });
	});

	it('explains a missing TMDB key', async () => {
		const body = { error: { code: 'tmdb_not_configured', message: 'x' } };
		const err = await fail(load(event('1396', answer(503, body))));
		expect(isHttpError(err, 503)).toBe(true);
		expect(err).toMatchObject({
			body: { message: expect.stringContaining('WATCHT_TMDB_API_KEY') }
		});
	});
});
