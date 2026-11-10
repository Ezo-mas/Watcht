import { describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import Page from './+error.svelte';

// The error page reads the failed page from $app/state: its status, error,
// route, and params. Each test sets them.
const { page } = vi.hoisted(() => ({
	page: {
		status: 404,
		error: null as { message: string; missing?: 'show' | 'season' | 'episode' } | null,
		route: { id: null as string | null },
		params: {} as Record<string, string>
	}
}));
vi.mock('$app/state', () => ({ page }));

/** The error page's HTML for a 404 on a route, as the server renders it (comments removed). */
function notFound(
	route: string,
	error: NonNullable<typeof page.error>,
	params: Record<string, string>
): string {
	Object.assign(page, { status: 404, route: { id: route }, error, params });
	return render(Page).body.replace(/<!--[^]*?-->/g, '');
}

const heading = (body: string) => body.match(/<h1[^>]*>([^<]*)<\/h1>/)?.[1];

/** The links as [href, text]. */
const links = (body: string) =>
	[...body.matchAll(/<a href="([^"]+)"[^>]*>([^<]*)<\/a>/g)].map((m) => [m[1], m[2]]);

describe('error page', () => {
	it('says which episode is missing, and links to its season and its show', () => {
		const body = notFound(
			'/(app)/shows/[id]/seasons/[n]/episodes/[e]',
			{ message: 'TMDB lists no episode S02E09 for Breaking Bad.', missing: 'episode' },
			{ id: '1396', n: '2', e: '9' }
		);
		expect(heading(body)).toBe('Episode not found');
		expect(links(body)).toEqual([
			['/shows/1396/seasons/2', 'Go to Season 2'],
			['/shows/1396', 'Go to the show'],
			['/', 'Go to the dashboard']
		]);
	});

	it('links an episode whose season number is unusable to the show only', () => {
		const body = notFound(
			'/(app)/shows/[id]/seasons/[n]/episodes/[e]',
			{ message: "There's no episode at this address.", missing: 'episode' },
			{ id: '1396', n: 'x', e: '9' }
		);
		expect(heading(body)).toBe('Episode not found');
		expect(links(body).map(([href]) => href)).toEqual(['/shows/1396', '/']);
	});

	it('says a missing season leads back to its show, not to the season', () => {
		const body = notFound(
			'/(app)/shows/[id]/seasons/[n]',
			{ message: 'TMDB lists no season 9 for Breaking Bad.', missing: 'season' },
			{ id: '1396', n: '9' }
		);
		expect(heading(body)).toBe('Season not found');
		expect(links(body).map(([href]) => href)).toEqual(['/shows/1396', '/']);
	});

	it('says a missing show is a title not found, even under a season address', () => {
		// The show's layout failed, so the route is the season's, but the show is what's missing.
		const body = notFound(
			'/(app)/shows/[id]/seasons/[n]',
			{ message: 'TMDB has no show with ID 9.', missing: 'show' },
			{ id: '9', n: '1' }
		);
		expect(heading(body)).toBe('Title not found');
		expect(links(body).map(([href]) => href)).toEqual(['/']);
	});

	it('still titles a movie by its route when the error says nothing', () => {
		const body = notFound(
			'/(app)/movies/[id]',
			{ message: 'TMDB has no movie with ID 9.' },
			{ id: '9' }
		);
		expect(heading(body)).toBe('Title not found');
	});
});
