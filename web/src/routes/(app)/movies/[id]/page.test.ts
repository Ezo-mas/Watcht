import { describe, expect, it } from 'vitest';
import { isHttpError } from '@sveltejs/kit';
import { render } from 'svelte/server';
import type { Movie, Watch } from '#lib/api/types.js';
import { formatWatch } from '#lib/format.js';
import { load } from './+page';
import Page from './+page.svelte';

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

const matrix: Movie = {
	tmdb_id: 603,
	imdb_id: 'tt0133093',
	title: 'The Matrix',
	original_title: 'The Matrix',
	original_language: 'en',
	overview: 'Set in the 22nd century, The Matrix tells the story of a computer hacker.',
	tagline: 'Believe the unbelievable.',
	status: 'Released',
	poster_path: '/p.jpg',
	poster_url: '',
	backdrop_path: '',
	backdrop_url: '',
	release_date: '1999-03-30',
	runtime: 136,
	genres: [
		{ tmdb_id: 28, name: 'Action' },
		{ tmdb_id: 878, name: 'Science Fiction' }
	],
	vote_average: 8.262,
	vote_count: 29114
};

const watches: Watch[] = [
	{ id: 2, watched_at: '2026-11-03T18:14:00Z', watched_on: null, source: 'manual' },
	{ id: 1, watched_at: '2025-12-24T12:00:00Z', watched_on: '2025-12-24', source: 'manual' }
];

/** The movie page's HTML, as the server renderer writes it (comments removed). */
function html(movie: Movie = matrix, list: Watch[] | null = watches): string {
	const props = { data: { movie, watches: list, watchesError: list ? '' : 'Server error' } };
	return render(Page, { props: props as never }).body.replace(/<!--[^]*?-->/g, '');
}

/** Every element of a tag whose attributes match, as text. */
const tags = (body: string, tag: string) => body.match(new RegExp(`<${tag}\\b[^>]*>`, 'g')) ?? [];

/** Every button: its opening tag and its text, tags removed and spaces collapsed. */
const buttons = (body: string) =>
	[...body.matchAll(/(<button\b[^>]*>)([^]*?)<\/button>/g)].map(([, tag, inner]) => ({
		tag,
		text: inner
			.replace(/<[^>]*>/g, ' ')
			.replace(/\s+/g, ' ')
			.trim()
	}));

describe('movie page', () => {
	it('keeps the facts, tagline, and overview', () => {
		const body = html();
		for (const label of ['Released', 'Runtime', 'Status', 'Genres', 'Language']) {
			expect(body).toMatch(new RegExp(`<dt[^>]*>\\s*${label}\\s*</dt>`));
		}
		expect(body).toContain('30 Mar 1999');
		expect(body).toContain('Action, Science Fiction');
		expect(body).toContain('Believe the unbelievable.');
		expect(body).toContain('Set in the 22nd century');
	});

	it('offers "Mark as watched" with a when-menu until it is watched', () => {
		const body = html(matrix, []);
		const mark = buttons(body).find((b) => b.text === 'Mark as watched');
		expect(mark?.tag).toContain('id="mark-movie"');
		const menu = buttons(body).find((b) =>
			b.tag.includes('aria-label="Choose when you watched The Matrix"')
		);
		expect(menu?.tag).toContain('aria-expanded="false"');
		expect(body).not.toMatch(/\d+ plays?\b/);
	});

	it('shows the play count and the latest play once watched, and no list of watches', () => {
		const body = html();
		const latest = formatWatch(watches[0]);
		const plays = buttons(body).find((b) => b.tag.includes('aria-haspopup="dialog"'));
		expect(plays?.text).toBe(`2 plays ${latest}`);
		expect(plays?.tag).toContain(
			`aria-label="2 plays, latest ${latest}. Your plays of The Matrix"`
		);
		expect(plays?.tag).toMatch(/popovertarget="[^"]+-plays"/);
		expect(body).toContain('aria-label="Add another play of The Matrix"');
		expect(body).not.toContain('Mark as watched');
		expect(body).not.toMatch(/Watched 2\s+times/);
		// The plays are only in the menu (a popover, hidden until opened),
		// not listed in the column under the button.
		const menuStart = body.indexOf('popover="auto"');
		expect(menuStart).toBeGreaterThan(body.indexOf('id="movie-plays"'));
		for (const w of watches) {
			const row = body.indexOf(`Remove the play from ${formatWatch(w)}`);
			expect(row).toBeGreaterThan(menuStart);
		}
	});

	it('lists every play in the plays menu, in a list that scrolls, with the actions fixed under it', () => {
		const many: Watch[] = Array.from({ length: 12 }, (_, i) => ({
			id: 100 - i,
			watched_at: new Date(Date.UTC(2026, 9, 30 - i, 20)).toISOString(),
			watched_on: null,
			source: 'manual'
		}));
		const body = html(matrix, many);
		// No header row (the button already shows the count): the dialog is
		// named instead, and the list comes first.
		const dialogStart = body.indexOf('popover="auto"');
		const dialogTag = body.slice(
			body.lastIndexOf('<div', dialogStart),
			body.indexOf('>', dialogStart) + 1
		);
		expect(dialogTag).toContain('role="dialog"');
		expect(dialogTag).toContain('aria-label="Plays of The Matrix"');
		expect(dialogTag).not.toContain('aria-labelledby');
		const dialog = body.slice(dialogStart, body.indexOf('View all plays', dialogStart));
		expect(dialog).not.toContain('aria-label="Close"');
		expect(dialog).not.toMatch(/>\s*(Your plays|Plays)\b/);
		expect(body.slice(body.indexOf('>', dialogStart) + 1)).toMatch(/^\s*<ul aria-label="Plays"/);
		const listStart = body.indexOf('aria-label="Plays"');
		const listEnd = body.indexOf('</ul>', listStart);
		const list = body.slice(listStart, listEnd);
		expect(list.match(/Remove the play from /g)).toHaveLength(12);
		for (const w of many) expect(list).toContain(formatWatch(w));
		// It scrolls inside the menu instead of growing past the window.
		const listTag = body.slice(body.lastIndexOf('<ul', listStart), listStart + 400);
		expect(listTag).toMatch(/overflow-y-auto/);
		expect(listTag).toMatch(/overscroll-contain/);
		expect(listTag).toMatch(/max-h-\[/);
		// "Remove all plays" and "View all plays" come after the list, not in it.
		expect(body.indexOf('Remove all plays', listEnd)).toBeGreaterThan(listEnd);
		expect(body.indexOf('View all plays', listEnd)).toBeGreaterThan(listEnd);
		expect(list).not.toContain('Remove all plays');
		const removeAll = buttons(body).find((b) => b.tag.includes('aria-label="Remove all plays"'));
		expect(removeAll?.tag).toContain('aria-disabled="true"');
		expect(body).toMatch(/<a href="\/history"[^>]*data-item/);
		expect(body).not.toContain('Keep it');
		expect(body).not.toContain('The newest');
	});

	it('still offers "Mark as watched" when the plays fail to load, with the error', () => {
		const body = html(matrix, null);
		expect(buttons(body).some((b) => b.text === 'Mark as watched')).toBe(true);
		expect(body).toContain("Couldn't load your plays of this movie: Server error");
		expect(buttons(body).some((b) => b.text === 'Try again')).toBe(true);
	});

	it('shows watchlist, rating, and where to watch as placeholders in the action column', () => {
		const later = buttons(html()).filter(
			(b) => b.tag.includes('aria-disabled="true"') && !b.tag.includes('aria-label=')
		);
		expect(later.map((b) => b.text)).toEqual(['Add to watchlist', 'Rate it', 'Where to watch']);
		for (const b of later) expect(b.tag).toMatch(/aria-describedby="[^"]+-later"/);
	});

	it("doesn't list the IMDb and TMDB IDs as facts", () => {
		const body = html();
		expect(body).not.toMatch(/<dt[^>]*>\s*(IMDb|TMDB)\s*<\/dt>/);
		expect(body).not.toMatch(/>\s*tt0133093\s*</);
	});

	it('links to IMDb and TMDB as icon links that open in a new tab', () => {
		const body = html();
		const imdb = tags(body, 'a').find((a) =>
			a.includes('href="https://www.imdb.com/title/tt0133093/"')
		);
		const tmdb = tags(body, 'a').find((a) =>
			a.includes('href="https://www.themoviedb.org/movie/603"')
		);
		for (const link of [imdb, tmdb]) {
			expect(link).toBeDefined();
			expect(link).toContain('target="_blank"');
			expect(link).toContain('rel="noopener noreferrer"');
		}
		expect(body).toContain('The Matrix on IMDb (opens in a new tab)');
		expect(body).toContain('The Matrix on TMDB (opens in a new tab)');
	});

	it('leaves out the IMDb link when TMDB has no IMDb ID', () => {
		const body = html({ ...matrix, imdb_id: '' });
		expect(body).not.toContain('imdb.com');
		expect(body).toContain('href="https://www.themoviedb.org/movie/603"');
	});

	it("shows TMDB's score in the ratings band, with the votes in its tooltip", () => {
		const body = html();
		const band = body.slice(
			body.indexOf('aria-label="Ratings"'),
			body.indexOf('</ul>', body.indexOf('aria-label="Ratings"'))
		);
		expect(band).toContain('TMDB rating 8.3 out of 10, from 29,114 votes');
		expect(band).toContain('title="29,114 votes on TMDB"');
		expect(band).toMatch(/aria-hidden="true"[^>]*>8\.3<\/span>/);
		expect(band).not.toContain('/ 10');
		// Votes only in the tooltip and the screen-reader text, not on screen.
		const visible = band.replace(/<span class="sr-only">[^<]*<\/span>/g, '');
		expect(visible).not.toMatch(/>[^<]*votes/);
		expect(body).not.toContain('All stats');
	});

	it('has no ratings band without votes', () => {
		const body = html({ ...matrix, vote_average: 0, vote_count: 0 });
		expect(body).not.toContain('aria-label="Ratings"');
		expect(body).not.toContain('TMDB rating');
	});

	it('has the sections under the overview, each in the section list', () => {
		const body = html();
		const sections = {
			stats: 'Stats',
			people: 'People',
			videos: 'Videos',
			recommendations: 'If you like The Matrix, check out…'
		};
		for (const [id, heading] of Object.entries(sections)) {
			expect(body).toMatch(new RegExp(`<section[^>]*id="${id}"`));
			expect(body).toMatch(new RegExp(`<h2[^>]*id="${id}-title"[^>]*>\\s*${heading}\\s*</h2>`));
			expect(body).toContain(`href="#${id}"`);
		}
		expect(body).toContain('id="overview"');
		expect(body).toContain('href="#overview"');
	});

	it('shows the unbuilt parts as placeholders, with no made-up data', () => {
		const body = html();
		expect(body).toContain('Cast and crew are coming in a later update.');
		expect(body).toContain('Trailers and clips are coming in a later update.');
		expect(body).toContain('Recommendations are coming in a later update.');
		// The section filters (the plays menu's own placeholder is tested above).
		const filters = tags(body.slice(body.indexOf('id="stats"')), 'button').filter(
			(b) => b.includes('aria-disabled="true"') && b.includes('aria-label=')
		);
		expect(filters.map((b) => b.match(/aria-label="([^"]*)"/)?.[1])).toEqual([
			'Show: cast',
			'Show: trailers'
		]);
		expect(filters[0]).toContain('aria-describedby="people-later"');
		// The only score in Stats is TMDB's own, straight under the heading.
		const stats = body.slice(body.indexOf('id="stats"'), body.indexOf('id="people"'));
		expect(stats).not.toContain('Second opinions');
		expect(stats).toMatch(/<ul aria-label="Ratings from other sites"/);
		expect(stats).toMatch(/<h3[^>]*>\s*Household ratings\s*<\/h3>/);
		expect(stats).toMatch(/<h3[^>]*>\s*Household activity\s*<\/h3>/);
		expect(stats).toContain('29,114 votes');
		expect(stats).toContain('8.3 out of 10');
		expect(stats).toContain('More sites’ scores are coming in a later update.');
		expect(stats.match(/\d+(\.\d+)? out of 10/g)).toEqual(['8.3 out of 10']);
	});
});
