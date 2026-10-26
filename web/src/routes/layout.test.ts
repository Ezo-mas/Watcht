import { describe, expect, it } from 'vitest';
import { isRedirect } from '@sveltejs/kit';
import { load } from './+layout';

// The root load: session check, redirects, and the unreachable state.

type LoadEvent = Parameters<typeof load>[0];

function event(path: string, fetchFn: typeof fetch): LoadEvent {
	return {
		url: new URL(path, 'http://localhost'),
		fetch: fetchFn,
		depends: () => {}
	} as unknown as LoadEvent;
}

const answer = (status: number, body: unknown) =>
	(async () =>
		new Response(typeof body === 'string' ? body : JSON.stringify(body), {
			status,
			headers: { 'Content-Type': 'application/json' }
		})) as unknown as typeof fetch;

const signedIn = { setup_required: false, user: { id: 1, username: 'a', is_admin: false } };

describe('root layout load', () => {
	it('returns the session when the page is allowed', async () => {
		const data = await load(event('/', answer(200, signedIn)));
		expect(data).toEqual({ session: signedIn, unreachable: null });
	});

	it('redirects as the guard says', async () => {
		const err = await Promise.resolve(
			load(event('/history', answer(200, { setup_required: false, user: null })))
		).catch((e: unknown) => e);
		expect(isRedirect(err)).toBe(true);
		expect(err).toMatchObject({ status: 307, location: '/login?next=%2Fhistory' });
	});

	it.each([
		[
			'no connection',
			(async () => {
				throw new TypeError('Failed to fetch');
			}) as unknown as typeof fetch,
			0
		],
		['a server error', answer(502, '<html>Bad Gateway</html>'), 502],
		['an answer that is not watcht', answer(200, '<html>portal</html>'), 0]
	])('shows the unreachable state for %s', async (_name, fetchFn, status) => {
		const data = await load(event('/', fetchFn));
		expect(data?.unreachable).toMatchObject({ status });
		// No redirect: the layout shows the problem in place.
	});

	it('throws other errors to the error page', async () => {
		const err = await Promise.resolve(
			load(event('/', answer(404, { error: { code: 'not_found', message: 'x' } })))
		).catch((e: unknown) => e);
		expect(err).toMatchObject({ status: 404, code: 'not_found' });
	});
});
