import { describe, expect, it } from 'vitest';
import { isRedirect } from '@sveltejs/kit';
import { load } from './+page';

type LoadEvent = Parameters<typeof load>[0];

function event(isAdmin: boolean, fetchFn: typeof fetch = fetch): LoadEvent {
	return {
		fetch: fetchFn,
		depends: () => {},
		parent: async () => ({
			session: { setup_required: false, user: { id: 1, username: 'a', is_admin: isAdmin } }
		})
	} as unknown as LoadEvent;
}

describe('admin users load', () => {
	it('sends non-admins to /settings without calling the API', async () => {
		let called = false;
		const fetchFn = (async () => {
			called = true;
			return new Response('{}');
		}) as unknown as typeof fetch;
		const err = await Promise.resolve(load(event(false, fetchFn))).catch((e: unknown) => e);
		expect(isRedirect(err)).toBe(true);
		expect(err).toMatchObject({ status: 307, location: '/settings' });
		expect(called).toBe(false);
	});

	it('loads the users for an admin', async () => {
		const users = [{ id: 1, username: 'a', is_admin: true, created_at: '2026-10-27T10:00:00Z' }];
		const fetchFn = (async () =>
			new Response(JSON.stringify({ users }), {
				headers: { 'Content-Type': 'application/json' }
			})) as unknown as typeof fetch;
		expect(await load(event(true, fetchFn))).toEqual({ users });
	});
});
