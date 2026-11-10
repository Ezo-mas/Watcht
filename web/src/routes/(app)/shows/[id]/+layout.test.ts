import { describe, expect, it } from 'vitest';
import { isHttpError } from '@sveltejs/kit';
import type { Show } from '#lib/api/types.js';
import { load } from './+layout';

type LoadEvent = Parameters<typeof load>[0];

const show = { tmdb_id: 1396, name: 'Breaking Bad', seasons: [] } as unknown as Show;

function event(id: string, fetchFn: typeof fetch): LoadEvent {
	return {
		params: { id },
		fetch: fetchFn,
		depends: () => {}
	} as unknown as LoadEvent;
}

const json = (status: number, body: unknown) =>
	(async () =>
		new Response(JSON.stringify(body), {
			status,
			headers: { 'Content-Type': 'application/json' }
		})) as unknown as typeof fetch;

const fail = async (p: unknown) => Promise.resolve(p).catch((e: unknown) => e);

describe('show layout load', () => {
	it('loads the show for its pages', async () => {
		expect(await load(event('1396', json(200, { show })))).toEqual({ show });
	});

	it("is a 404 for an address that isn't a show ID, saying the show is missing", async () => {
		const err = await fail(load(event('x', json(200, {}))));
		expect(isHttpError(err, 404)).toBe(true);
		expect(err).toMatchObject({
			body: { message: "There's no show at this address.", missing: 'show' }
		});
	});

	it('is a 404 naming the show when TMDB has none', async () => {
		const err = await fail(
			load(event('9', json(404, { error: { code: 'not_found', message: 'x' } })))
		);
		expect(isHttpError(err, 404)).toBe(true);
		expect(err).toMatchObject({
			body: { message: 'TMDB has no show with ID 9.', missing: 'show' }
		});
	});
});
