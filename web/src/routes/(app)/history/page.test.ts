import { describe, expect, it } from 'vitest';
import { isHttpError } from '@sveltejs/kit';
import { load } from './+page';

type LoadEvent = Parameters<typeof load>[0];

function event(fetchFn: typeof fetch): LoadEvent {
	return { fetch: fetchFn, depends: () => {} } as unknown as LoadEvent;
}

const answer = (status: number, body: unknown, seen: string[] = []) =>
	(async (input: RequestInfo | URL) => {
		seen.push(String(input));
		return new Response(JSON.stringify(body), {
			status,
			headers: { 'Content-Type': 'application/json' }
		});
	}) as unknown as typeof fetch;

const fail = async (p: unknown) => Promise.resolve(p).catch((e: unknown) => e);

describe('history page load', () => {
	it('loads the first page, with the default page size', async () => {
		const history = {
			entries: [
				{
					id: 3,
					watched_at: '2026-11-03T18:14:00Z',
					source: 'plex',
					movie: {
						tmdb_id: 603,
						title: 'The Matrix',
						poster_path: '/p.jpg',
						poster_url: 'https://image.tmdb.org/t/p/w342/p.jpg',
						release_date: '1999-03-30',
						runtime: 136,
						vote_average: 8.2
					},
					episode: null
				}
			],
			next_cursor: '1762193640_3'
		};
		const seen: string[] = [];
		expect(await load(event(answer(200, history, seen)))).toEqual({ history });
		expect(seen).toEqual(['/api/v1/history']);
	});

	it('turns a failure into an error page with the message', async () => {
		const err = await fail(
			load(event(answer(500, { error: { code: 'internal', message: 'database is locked' } })))
		);
		expect(isHttpError(err, 500)).toBe(true);
		expect(err).toMatchObject({ body: { message: 'database is locked' } });
	});
});
