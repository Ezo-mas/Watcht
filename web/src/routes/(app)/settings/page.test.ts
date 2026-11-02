import { describe, expect, it } from 'vitest';
import { isHttpError } from '@sveltejs/kit';
import { load } from './+page';

type LoadEvent = Parameters<typeof load>[0];

const event = (fetchFn: typeof fetch) =>
	({ fetch: fetchFn, depends: () => {} }) as unknown as LoadEvent;

describe('settings load', () => {
	it('loads the tokens', async () => {
		const tokens = [{ id: 1, name: 'Jellyfin', created_at: '2026-10-27T10:00:00Z' }];
		const fetchFn = (async () =>
			new Response(JSON.stringify({ tokens }), {
				headers: { 'Content-Type': 'application/json' }
			})) as unknown as typeof fetch;
		expect(await load(event(fetchFn))).toEqual({ tokens });
	});

	it('turns an unreachable server into a 503 with a useful message', async () => {
		const fetchFn = (async () => {
			throw new TypeError('Failed to fetch');
		}) as unknown as typeof fetch;
		const err = await Promise.resolve(load(event(fetchFn))).catch((e: unknown) => e);
		expect(isHttpError(err, 503)).toBe(true);
		expect(err).toMatchObject({ body: { message: expect.stringContaining("Can't reach") } });
	});
});
