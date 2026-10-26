import type { SessionStatus } from './api/types';

/** Pages reachable without signing in. */
const publicPaths = new Set(['/login', '/setup']);

/**
 * Decides where a page load must go instead, or null to stay:
 * - before first-run setup, everything goes to /setup;
 * - signed out, everything but /login goes there (remembering the page);
 * - signed in, /login and /setup go to the dashboard.
 */
export function guard(session: SessionStatus, url: URL): string | null {
	// Routes also match with a trailing slash ("/login/"), so compare
	// without one.
	const path = url.pathname.length > 1 ? url.pathname.replace(/\/+$/, '') : url.pathname;
	if (session.setup_required) return path === '/setup' ? null : '/setup';
	if (session.user) return publicPaths.has(path) ? '/' : null;
	if (path === '/login') return null;
	const back = path + url.search;
	return back === '/' || path === '/setup' ? '/login' : `/login?next=${encodeURIComponent(back)}`;
}

/**
 * Returns where to go after signing in: the ?next= page if it's a path on
 * this site, otherwise the dashboard. Anything that would leave the site
 * ("//evil.example", "https://…", "/\evil") is refused.
 */
export function safeNext(next: string | null): string {
	if (!next || !next.startsWith('/') || next.startsWith('//') || next.includes('\\')) return '/';
	const base = 'http://watcht.invalid';
	let url: URL;
	try {
		url = new URL(next, base);
	} catch {
		return '/';
	}
	// Dot segments can collapse into "//" ("/.//evil.example"), which a
	// browser would read as another host: check the result, not just the
	// input.
	if (url.origin !== base || url.pathname.startsWith('//')) return '/';
	return url.pathname + url.search + url.hash;
}
