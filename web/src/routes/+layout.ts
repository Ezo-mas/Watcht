import { redirect } from '@sveltejs/kit';
import { getSession } from '#lib/api/auth.js';
import { ApiError } from '#lib/api/client.js';
import { guard } from '#lib/guard.js';
import type { LayoutLoad } from './$types';

// SPA only: Go serves the built files, so nothing is rendered on a server or at build time.
export const ssr = false;
export const prerender = false;

// Every navigation checks the session and sends the user where they
// belong: /setup on a fresh instance, /login when signed out. The server
// enforces access on every API call; this only keeps the UI honest.
//
// If the server can't be reached or fails, the layout shows that itself
// (unreachable) instead of throwing: an error page would re-run this load
// and fail the same way.
export const load: LayoutLoad = async ({ url, fetch, depends }) => {
	depends('app:session');
	let session;
	try {
		session = await getSession(fetch);
	} catch (e) {
		// Status 0: no answer, or not watcht's (bad_response).
		if (e instanceof ApiError && (e.status === 0 || e.status >= 500)) {
			return { session: { setup_required: false, user: null }, unreachable: e };
		}
		throw e;
	}
	const to = guard(session, url);
	if (to) redirect(307, to);
	return { session, unreachable: null };
};
