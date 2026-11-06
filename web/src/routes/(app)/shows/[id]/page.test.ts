import { describe, expect, it } from 'vitest';
import { isHttpError } from '@sveltejs/kit';
import { render } from 'svelte/server';
import type { Show } from '#lib/api/types.js';
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

const breakingBad: Show = {
	tmdb_id: 1396,
	imdb_id: 'tt0903747',
	tvdb_id: 81189,
	name: 'Breaking Bad',
	original_name: 'Breaking Bad',
	original_language: 'en',
	overview: 'Walter White, a New Mexico chemistry teacher, is diagnosed with cancer.',
	tagline: 'Change the equation.',
	status: 'Ended',
	poster_path: '',
	poster_url: '',
	backdrop_path: '',
	backdrop_url: '',
	first_air_date: '2008-01-20',
	last_air_date: '2013-09-29',
	in_production: false,
	genres: [{ tmdb_id: 18, name: 'Drama' }],
	seasons: [],
	vote_average: 8.9,
	vote_count: 15000
};

/** The show page's HTML, as the server renderer writes it (comments removed). */
function html(show: Show = breakingBad): string {
	const props = { data: { show, watched: [], watchedError: '' } };
	return render(Page, { props: props as never }).body.replace(/<!--[^]*?-->/g, '');
}

describe('show page', () => {
	it('keeps its facts, overview, and mark button, without the ID facts', () => {
		const body = html();
		for (const label of ['Aired', 'Status', 'Genres', 'Language']) {
			expect(body).toMatch(new RegExp(`<dt[^>]*>\\s*${label}\\s*</dt>`));
		}
		expect(body).toContain('Change the equation.');
		expect(body).toContain('Mark show watched');
		expect(body).not.toMatch(/<dt[^>]*>\s*(IMDb|TMDB)\s*<\/dt>/);
		expect(body).not.toMatch(/>\s*tt0903747\s*</);
	});

	it('links to IMDb and TMDB as icon links', () => {
		const body = html();
		expect(body).toContain('href="https://www.imdb.com/title/tt0903747/"');
		expect(body).toContain('href="https://www.themoviedb.org/tv/1396"');
		expect(body).toContain('Breaking Bad on IMDb (opens in a new tab)');
		expect(body).toContain('Breaking Bad on TMDB (opens in a new tab)');
	});

	it("shows TMDB's score in the ratings band", () => {
		expect(html()).toContain('TMDB rating 8.9 out of 10, from 15,000 votes');
	});

	it('lists Overview and Seasons, and none of the movie placeholder sections', () => {
		const body = html();
		expect(body).toContain('href="#overview"');
		expect(body).toContain('href="#seasons"');
		expect(body).toMatch(/<h2[^>]*id="seasons-title"/);
		for (const id of ['stats', 'people', 'videos', 'recommendations']) {
			expect(body).not.toContain(`id="${id}"`);
		}
		// No stats section, so the band has no link to one.
		expect(body).not.toContain('href="#stats"');
	});
});
